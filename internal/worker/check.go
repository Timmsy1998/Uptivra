package worker

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"uptivra/internal/domain"
	"uptivra/internal/netguard"
)

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

func Check(ctx context.Context, m domain.Monitor) domain.Check {
	return Execute(ctx, m, netguard.Client(time.Duration(m.Timeout)*time.Second))
}
func Execute(ctx context.Context, m domain.Monitor, client Doer) domain.Check {
	start := time.Now()
	c := domain.Check{At: start, Reason: "network_error"}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(m.Timeout)*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", m.URL, nil)
	if e != nil {
		return c
	}
	req.Header.Set("User-Agent", "Uptivra/1.0 (+https://uptivra)")
	for k, v := range m.Headers {
		req.Header.Set(k, v)
	}
	res, e := client.Do(req)
	if e != nil {
		c.Latency = float64(time.Since(start).Microseconds()) / 1000
		if ctx.Err() != nil {
			c.Reason = "timeout"
		}
		return c
	}
	defer res.Body.Close()
	c.Code = res.StatusCode
	c.OK = c.Code == m.ExpectedStatus
	c.Reason = "unexpected_status"
	if c.OK && m.Contains != "" {
		body, e := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
		if e != nil {
			c.OK = false
			c.Reason = "body_error"
		} else if len(body) > 1<<20 {
			c.OK = false
			c.Reason = "body_limit"
		} else if !strings.Contains(string(body), m.Contains) {
			c.OK = false
			c.Reason = "content_mismatch"
		}
	}
	c.Latency = float64(time.Since(start).Microseconds()) / 1000
	if c.OK {
		c.Reason = ""
	}
	return c
}
