package bib

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// errNotFound is a sentinel error indicating the identifier does not exist
// in the remote service (e.g. Crossref 404, empty arXiv feed).
var errNotFound = errors.New("identifier not found")

const (
	maxRetries   = 3
	baseDelay    = 1 * time.Second
	maxRetryWait = 30 * time.Second
)

// retryableStatusCode reports whether the HTTP status code warrants a retry.
func retryableStatusCode(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

// isRetryableError reports whether err is a transient network error (timeout).
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return true
	}
	return false
}

// friendlyHTTPError returns a user-friendly error message for common HTTP errors.
func friendlyHTTPError(code int, service string) error {
	switch {
	case code == http.StatusNotFound:
		return fmt.Errorf("not found in %s: %w", service, errNotFound)
	case code == http.StatusTooManyRequests:
		return fmt.Errorf("%s rate limited, try again later", service)
	case code >= 500:
		return fmt.Errorf("%s server error, try again later", service)
	default:
		return fmt.Errorf("%s returned HTTP %d", service, code)
	}
}

// doWithRetry executes fn up to maxRetries times with exponential backoff.
// Retries only on HTTP 429, 5xx, and timeout errors.
func doWithRetry(fn func() (*http.Response, error), log Logger, key string) (*http.Response, error) {
	var lastErr error
	delay := baseDelay
	for attempt := range maxRetries {
		resp, err := fn()
		if err != nil {
			if !isRetryableError(err) || attempt == maxRetries-1 {
				return nil, err
			}
			lastErr = err
			log.Warn(key, fmt.Sprintf("request timed out, retrying in %s...", delay))
			time.Sleep(delay)
			delay *= 2
			continue
		}
		if !retryableStatusCode(resp.StatusCode) {
			return resp, nil
		}
		// Retryable HTTP status — close body before retry.
		resp.Body.Close()
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		if attempt == maxRetries-1 {
			break
		}
		// Honor Retry-After header on 429.
		if resp.StatusCode == http.StatusTooManyRequests {
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(ra); err == nil && time.Duration(secs)*time.Second < maxRetryWait {
					delay = time.Duration(secs) * time.Second
				}
			}
		}
		log.Warn(key, fmt.Sprintf("HTTP %d, retrying in %s...", resp.StatusCode, delay))
		time.Sleep(delay)
		delay *= 2
	}
	return nil, lastErr
}
