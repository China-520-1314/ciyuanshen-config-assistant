package main

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Honor server backoff without holding a request indefinitely. Longer delays
// are returned to the caller as the original HTTP error rather than retried early.
func routerRetryDelay(header string, attempt int, now time.Time) (time.Duration, bool) {
	delay := time.Duration(attempt+1) * 500 * time.Millisecond
	if value := strings.TrimSpace(header); value != "" {
		if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 {
			if seconds > 30 {
				return 0, false
			}
			delay = time.Duration(seconds) * time.Second
		} else if date, err := http.ParseTime(value); err == nil {
			delay = date.Sub(now)
			if delay < 0 {
				delay = 0
			}
		}
	}
	return delay, delay <= 30*time.Second
}

func (p *modelRouter) doChatRequest(ctx context.Context, upstream, key string, body []byte) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", upstream+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, err
		}
		if attempt == 2 || (resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusServiceUnavailable) {
			return resp, nil
		}
		delay, retry := routerRetryDelay(resp.Header.Get("Retry-After"), attempt, time.Now())
		if !retry {
			return resp, nil
		}
		resp.Body.Close()
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
