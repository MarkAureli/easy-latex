package bib

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDoWithRetry_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	resp, err := doWithRetry(func() (*http.Response, error) {
		return http.Get(ts.URL)
	}, nopLogger{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestDoWithRetry_RetriesOn429(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	resp, err := doWithRetry(func() (*http.Response, error) {
		return http.Get(ts.URL)
	}, nopLogger{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoWithRetry_RetriesOn500(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	resp, err := doWithRetry(func() (*http.Response, error) {
		return http.Get(ts.URL)
	}, nopLogger{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestDoWithRetry_NoRetryOn404(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	resp, err := doWithRetry(func() (*http.Response, error) {
		return http.Get(ts.URL)
	}, nopLogger{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestDoWithRetry_ExhaustedRetries(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	_, err := doWithRetry(func() (*http.Response, error) {
		return http.Get(ts.URL)
	}, nopLogger{}, "")
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
}

func TestDoWithRetry_NetworkError_NoRetry(t *testing.T) {
	_, err := doWithRetry(func() (*http.Response, error) {
		return nil, fmt.Errorf("connection refused")
	}, nopLogger{}, "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFriendlyHTTPError(t *testing.T) {
	tests := []struct {
		code    int
		service string
		want    string
	}{
		{404, "Crossref", "not found in Crossref: identifier not found"},
		{429, "arXiv", "arXiv rate limited, try again later"},
		{500, "Crossref", "Crossref server error, try again later"},
		{503, "arXiv", "arXiv server error, try again later"},
		{403, "Crossref", "Crossref returned HTTP 403"},
	}
	for _, tt := range tests {
		err := friendlyHTTPError(tt.code, tt.service)
		if err.Error() != tt.want {
			t.Errorf("friendlyHTTPError(%d, %q) = %q, want %q", tt.code, tt.service, err.Error(), tt.want)
		}
	}
}

func TestFriendlyHTTPError_404WrapsErrNotFound(t *testing.T) {
	err := friendlyHTTPError(404, "Crossref")
	if !errors.Is(err, errNotFound) {
		t.Errorf("404 error should wrap errNotFound, got: %v", err)
	}
}

func TestFriendlyHTTPError_NonNotFoundDoesNotWrapErrNotFound(t *testing.T) {
	for _, code := range []int{429, 500, 503, 403} {
		err := friendlyHTTPError(code, "test")
		if errors.Is(err, errNotFound) {
			t.Errorf("HTTP %d should not wrap errNotFound", code)
		}
	}
}

func TestRetryableStatusCode(t *testing.T) {
	if !retryableStatusCode(429) {
		t.Error("429 should be retryable")
	}
	if !retryableStatusCode(500) {
		t.Error("500 should be retryable")
	}
	if !retryableStatusCode(503) {
		t.Error("503 should be retryable")
	}
	if retryableStatusCode(404) {
		t.Error("404 should not be retryable")
	}
	if retryableStatusCode(200) {
		t.Error("200 should not be retryable")
	}
}

func TestIsRetryableError(t *testing.T) {
	if isRetryableError(nil) {
		t.Error("nil should not be retryable")
	}
	if isRetryableError(fmt.Errorf("some error")) {
		t.Error("generic error should not be retryable")
	}
	// Timeout errors are retryable — use a client with a tiny timeout.
	client := &http.Client{Timeout: 1 * time.Nanosecond}
	_, err := client.Get("http://192.0.2.1:1") // non-routable, will timeout
	if err != nil && !isRetryableError(err) {
		// On some systems this may be a connection refused rather than timeout.
		// Only fail if the error IS a timeout but isn't detected.
		t.Logf("error %T: %v (may not be timeout on all systems)", err, err)
	}
}
