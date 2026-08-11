package provider

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryTransport_RetriesServerErrorsAndReplaysBody(t *testing.T) {
	var attempts int32
	var bodies []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		bodies = append(bodies, string(body))

		if atomic.AddInt32(&attempts, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Transport: &retryTransport{base: http.DefaultTransport, maxRetries: 2}}
	resp, err := client.Post(server.URL, "application/json", strings.NewReader(`{"runnerId":"r-1"}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))
	assert.Equal(t, []string{`{"runnerId":"r-1"}`, `{"runnerId":"r-1"}`, `{"runnerId":"r-1"}`}, bodies)
}

func TestRetryTransport_DoesNotRetryClientErrors(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := &http.Client{Transport: &retryTransport{base: http.DefaultTransport, maxRetries: 2}}
	resp, err := client.Post(server.URL, "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestRetryTransport_GivesUpAfterMaxRetries(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := &http.Client{Transport: &retryTransport{base: http.DefaultTransport, maxRetries: 1}}
	resp, err := client.Post(server.URL, "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, int32(2), atomic.LoadInt32(&attempts))
}

func TestRetryTransport_ZeroMaxRetriesSendsOnce(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := &http.Client{Transport: &retryTransport{base: http.DefaultTransport, maxRetries: 0}}
	resp, err := client.Post(server.URL, "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestRetryTransport_HonorsShouldRetryHeader(t *testing.T) {
	t.Run("false suppresses a retryable status", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.Header().Set("x-should-retry", "false")
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := &http.Client{Transport: &retryTransport{base: http.DefaultTransport, maxRetries: 3}}
		resp, err := client.Post(server.URL, "application/json", strings.NewReader(`{}`))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, int32(1), atomic.LoadInt32(&attempts))
	})

	t.Run("true forces a retry on a non-retryable status", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.Header().Set("x-should-retry", "true")
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		client := &http.Client{Transport: &retryTransport{base: http.DefaultTransport, maxRetries: 1}}
		resp, err := client.Post(server.URL, "application/json", strings.NewReader(`{}`))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, int32(2), atomic.LoadInt32(&attempts))
	})
}

func TestRetryTransport_RetriesConflictAndRequestTimeout(t *testing.T) {
	for _, status := range []int{http.StatusRequestTimeout, http.StatusConflict} {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(status)
		}))

		client := &http.Client{Transport: &retryTransport{base: http.DefaultTransport, maxRetries: 1}}
		resp, err := client.Post(server.URL, "application/json", strings.NewReader(`{}`))
		require.NoError(t, err)
		_ = resp.Body.Close()
		server.Close()

		assert.Equal(t, int32(2), atomic.LoadInt32(&attempts), "status %d should be retried", status)
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Run("prefers the v1 RateLimit-RetryAfter header", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("RateLimit-RetryAfter", "5")
		resp.Header.Set("Retry-After", "30")

		got, ok := parseRetryAfter(resp)
		require.True(t, ok)
		assert.Equal(t, 5*time.Second, got)
	})

	t.Run("prefers Retry-After-Ms", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After-Ms", "250")
		resp.Header.Set("Retry-After", "30")

		got, ok := parseRetryAfter(resp)
		require.True(t, ok)
		assert.Equal(t, 250*time.Millisecond, got)
	})

	t.Run("reads Retry-After seconds", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After", "3")

		got, ok := parseRetryAfter(resp)
		require.True(t, ok)
		assert.Equal(t, 3*time.Second, got)
	})

	t.Run("ignores an implausibly long wait", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After", "3600")

		_, ok := parseRetryAfter(resp)
		assert.False(t, ok)
	})

	t.Run("ignores a past HTTP-date", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After", time.Now().Add(-time.Hour).UTC().Format(time.RFC1123))

		_, ok := parseRetryAfter(resp)
		assert.False(t, ok)
	})

	t.Run("no header", func(t *testing.T) {
		_, ok := parseRetryAfter(&http.Response{Header: http.Header{}})
		assert.False(t, ok)
	})
}

// request_timeout is documented as per-attempt, so a slow first attempt must not
// eat the budget of the retries that follow it.
func TestRetryTransport_TimeoutAppliesPerAttempt(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			select {
			case <-time.After(2 * time.Second):
			case <-r.Context().Done():
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Transport: &retryTransport{
		base:              http.DefaultTransport,
		maxRetries:        1,
		perAttemptTimeout: 200 * time.Millisecond,
	}}

	resp, err := client.Post(server.URL, "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(2), atomic.LoadInt32(&attempts))
}

// The per-attempt deadline must survive until the caller has read the body.
func TestRetryTransport_BodyReadableAfterAttemptDeadlineSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"runnerId":"r-1"}`))
	}))
	defer server.Close()

	client := &http.Client{Transport: &retryTransport{
		base:              http.DefaultTransport,
		maxRetries:        2,
		perAttemptTimeout: 5 * time.Second,
	}}

	resp, err := client.Post(server.URL, "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"runnerId":"r-1"}`, string(body))
}

// A retryable response with an endless body must not stall the next attempt.
func TestRetryTransport_DrainOfEndlessBodyDoesNotStallRetry(t *testing.T) {
	var attempts int32
	release := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			flusher, ok := w.(http.Flusher)
			require.True(t, ok)
			for {
				select {
				case <-release:
					return
				case <-r.Context().Done():
					return
				default:
				}
				_, _ = w.Write(make([]byte, 1024))
				flusher.Flush()
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer func() {
		close(release)
		server.Close()
	}()

	client := &http.Client{Transport: &retryTransport{base: http.DefaultTransport, maxRetries: 1}}

	done := make(chan *http.Response, 1)
	go func() {
		resp, err := client.Post(server.URL, "application/json", strings.NewReader(`{}`))
		require.NoError(t, err)
		done <- resp
	}()

	select {
	case resp := <-done:
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, int32(2), atomic.LoadInt32(&attempts))
	case <-time.After(20 * time.Second):
		t.Fatal("retry stalled while draining the discarded response body")
	}
}
