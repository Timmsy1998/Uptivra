// Fetch public status without sharing authentication or a database with Uptivra.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

type Status struct {
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

func FetchStatus(ctx context.Context, origin string) (Status, error) {
	var status Status
	base, err := url.Parse(origin)
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") {
		return status, fmt.Errorf("UPTIVRA_ORIGIN must be an HTTP(S) origin")
	}
	endpoint := base.ResolveReference(&url.URL{Path: "/api/v1/public/status"})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return status, err
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return status, fmt.Errorf("status unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return status, fmt.Errorf("status unavailable: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return status, err
	}
	if len(body) > 1<<20 {
		return status, fmt.Errorf("status response too large")
	}
	if err = json.Unmarshal(body, &status); err != nil {
		return status, err
	}
	if status.Status == "" || status.UpdatedAt.IsZero() {
		return status, fmt.Errorf("invalid status response")
	}
	return status, nil
}

func main() {
	status, err := FetchStatus(context.Background(), os.Getenv("UPTIVRA_ORIGIN"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Service health: unknown")
		os.Exit(1)
	}
	fmt.Printf("%s: %s (updated %s)\n", status.Title, status.Status, status.UpdatedAt.Format(time.RFC3339))
}
