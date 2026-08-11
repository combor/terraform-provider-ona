package provider

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"time"
)

// defaultMaxRetries matches the retry count the provider used before migrating
// to the connect-based SDK, which has no retry support of its own.
const defaultMaxRetries = 2

const (
	retryBaseDelay = 100 * time.Millisecond
	retryMaxDelay  = 2 * time.Second

	// Bounds on draining a response that is about to be discarded and retried.
	maxDrainBytes    = 4 << 10
	maxDrainDuration = 2 * time.Second
)

// retryTransport retries idempotent-in-practice API calls that fail with a
// network error or a retryable status code, and applies perAttemptTimeout to
// each individual attempt. Every connect RPC is a POST with a fully buffered
// body, so the request can safely be replayed.
type retryTransport struct {
	base              http.RoundTripper
	maxRetries        int
	perAttemptTimeout time.Duration
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	// A negative ContentLength means a streaming body that cannot be replayed.
	retries := t.maxRetries
	if req.ContentLength < 0 {
		retries = 0
	}

	var body []byte
	if req.Body != nil && retries > 0 {
		var err error
		body, err = io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
	}

	for attempt := 0; ; attempt++ {
		attemptReq := req
		if body != nil {
			attemptReq = cloneWithBody(req, body)
		}

		ctx, cancel := t.attemptContext(req.Context())
		resp, err := base.RoundTrip(attemptReq.WithContext(ctx))

		if attempt >= retries || !shouldRetry(resp, err) {
			if resp == nil {
				cancel()
				return nil, err
			}
			// The caller still has to read the body, so the attempt deadline
			// must stay alive until that body is closed.
			resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}
			return resp, err
		}

		delay := retryDelay(attempt, resp)
		if resp != nil {
			drainBeforeRetry(resp, cancel)
		}
		cancel()

		select {
		case <-time.After(delay):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
}

// drainBeforeRetry reads the discarded response so the connection can be
// reused, bounded in both size and time: without request_timeout the attempt
// context has no deadline, so a large or slow error body would otherwise stall
// the retry indefinitely. Cancelling the attempt unblocks a stuck read.
func drainBeforeRetry(resp *http.Response, cancel context.CancelFunc) {
	timer := time.AfterFunc(maxDrainDuration, cancel)
	defer timer.Stop()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))
	_ = resp.Body.Close()
}

// attemptContext derives the deadline for a single attempt. The returned cancel
// is always non-nil and must be called once the attempt's response body is done.
func (t *retryTransport) attemptContext(parent context.Context) (context.Context, context.CancelFunc) {
	if t.perAttemptTimeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, t.perAttemptTimeout)
}

// cancelOnCloseBody releases the attempt context once the response body is
// closed, so a per-attempt deadline does not cut the body short mid-read.
type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func cloneWithBody(req *http.Request, body []byte) *http.Request {
	clone := req.Clone(req.Context())
	clone.Body = io.NopCloser(bytes.NewReader(body))
	clone.ContentLength = int64(len(body))
	clone.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	return clone
}

// shouldRetry mirrors the retry rules of the REST SDK the provider used before
// the connect migration: an explicit x-should-retry header wins over the status
// code, and only a fixed set of statuses is retried otherwise.
func shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if resp == nil {
		return true
	}

	switch resp.Header.Get("x-should-retry") {
	case "true":
		return true
	case "false":
		return false
	}

	return resp.StatusCode == http.StatusRequestTimeout ||
		resp.StatusCode == http.StatusConflict ||
		resp.StatusCode == http.StatusTooManyRequests ||
		resp.StatusCode >= http.StatusInternalServerError
}

func retryDelay(attempt int, resp *http.Response) time.Duration {
	if after, ok := parseRetryAfter(resp); ok {
		return after
	}

	delay := retryBaseDelay << attempt
	if delay > retryMaxDelay || delay <= 0 {
		return retryMaxDelay
	}
	return delay
}

// parseRetryAfter reads the server's requested wait for a rate-limited call.
// The v1 API signals this with RateLimit-RetryAfter (seconds); Retry-After-Ms
// and Retry-After are the older REST spellings, with Retry-After expressed
// either in seconds or as an HTTP-date.
//
// The API also carries the delay in a RetryInfo error detail, which lives in
// the connect error body and so is not readable from this transport.
func parseRetryAfter(resp *http.Response) (time.Duration, bool) {
	if resp == nil {
		return 0, false
	}

	for _, header := range []struct {
		name  string
		units time.Duration
	}{
		{name: "RateLimit-RetryAfter", units: time.Second},
		{name: "Retry-After-Ms", units: time.Millisecond},
		{name: "Retry-After", units: time.Second},
	} {
		value := resp.Header.Get(header.name)
		if value == "" {
			continue
		}

		if seconds, err := strconv.ParseFloat(value, 64); err == nil {
			return durationFromRetryAfter(time.Duration(seconds * float64(header.units)))
		}
		if header.name == "Retry-After" {
			if at, err := time.Parse(time.RFC1123, value); err == nil {
				return durationFromRetryAfter(time.Until(at))
			}
		}
	}

	return 0, false
}

// durationFromRetryAfter drops values that would make the provider hang or
// busy-loop: a past date or a wait longer than the request could reasonably
// tolerate is better handled by the normal backoff.
func durationFromRetryAfter(d time.Duration) (time.Duration, bool) {
	const maxRetryAfter = time.Minute

	if d <= 0 || d > maxRetryAfter {
		return 0, false
	}
	return d, true
}
