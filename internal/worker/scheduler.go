package worker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"uptivra/internal/domain"
	"uptivra/internal/netguard"
	"uptivra/internal/store"
)

type Report struct {
	Claimed   int   `json:"claimed"`
	Recorded  int64 `json:"recorded"`
	Delivered int64 `json:"delivered"`
}

// Tick waits for all work; no goroutines survive a serverless invocation.
func Tick(ctx context.Context, s *store.Store, appOrigin string) (Report, error) {
	var report Report
	ctx, cancel := context.WithTimeout(ctx, 55*time.Second)
	defer cancel()
	ms, e := s.Claim(ctx)
	if e != nil {
		return report, e
	}
	report.Claimed = len(ms)
	checkCtx, stop := context.WithTimeout(ctx, 42*time.Second)
	var recorded atomic.Int64
	var first error
	var mu sync.Mutex
	fail := func(e error) {
		if e != nil {
			mu.Lock()
			if first == nil {
				first = e
			}
			mu.Unlock()
		}
	}
	parallel(len(ms), 8, func(i int) {
		if checkCtx.Err() != nil {
			return
		}
		c := domain.Check{At: time.Now()}
		if ms[i].Type != "heartbeat" {
			c = Check(checkCtx, ms[i])
		}
		if checkCtx.Err() != nil {
			return
		}
		if e := s.Record(checkCtx, ms[i], c); e != nil {
			fail(e)
		} else {
			recorded.Add(1)
		}
	})
	stop()
	report.Recorded = recorded.Load()
	ds, e := s.ClaimDeliveries(ctx)
	if e != nil {
		return report, e
	}
	var delivered atomic.Int64
	parallel(len(ds), 8, func(i int) {
		d := ds[i]
		ok := Send(ctx, d, netguard.Client(5*time.Second), strings.TrimSuffix(appOrigin, "/")+"/status")
		if e := s.FinishDelivery(ctx, d, ok); e != nil {
			fail(e)
		} else if ok {
			delivered.Add(1)
		}
	})
	report.Delivered = delivered.Load()
	fail(s.Cleanup(ctx))
	return report, first
}
func parallel(n, workers int, f func(int)) {
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range jobs {
				f(i)
			}
		})
	}
	for i := range n {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

// Signature covers the exact body and a timestamp; receivers should reject old timestamps.
func Signature(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
func Send(ctx context.Context, d store.Delivery, client Doer, statusPageURL string) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	body := []byte(d.Payload)
	if d.Channel.Kind == "discord" {
		var e error
		body, e = discordPayload(body, statusPageURL)
		if e != nil {
			return false
		}
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, d.Channel.URL, bytes.NewReader(body))
	if e != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Uptivra/1.0")
	req.Header.Set("X-Uptivra-Delivery", d.ID)
	if d.Channel.Kind == "webhook" {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		req.Header.Set("X-Uptivra-Timestamp", ts)
		req.Header.Set("X-Uptivra-Signature", Signature(d.Channel.SigningSecret, ts, body))
	}
	resp, e := client.Do(req)
	if e != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
