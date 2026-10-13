// Package httpclient builds the HTTP clients every Harness SDK client uses, with one retry
// policy, one timeout and one (redacting) debug log, instead of the per-client defaults the
// SDKs ship with (retry everything including POST, no timeouts, verbatim body dumps).
package httpclient

import (
	"context"
	"crypto/x509"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-retryablehttp"
)

// Config configures the clients.
type Config struct {
	// MaxRetries is the number of retries after the first attempt for retryable failures.
	MaxRetries int
	// RequestTimeout bounds a single attempt; zero means no timeout.
	RequestTimeout time.Duration
	// UserAgent is sent with every request when set.
	UserAgent string
	// Transport is the base RoundTripper; the default is a pooled cleanhttp transport.
	Transport http.RoundTripper
	// Secrets are literal values redacted from debug logs (API keys).
	Secrets []string
}

const (
	retryWaitMin = 1 * time.Second
	retryWaitMax = 30 * time.Second
)

func (cfg Config) base() http.RoundTripper {
	if cfg.Transport != nil {
		return cfg.Transport
	}
	return cleanhttp.DefaultPooledTransport()
}

// NewPlain returns an http.Client with the timeout and debug logging but no retries, for
// SDKs that implement their own retry layer (Split).
func NewPlain(cfg Config) *http.Client {
	return &http.Client{
		Transport: NewLoggingTransport(cfg.base(), cfg.UserAgent, cfg.Secrets),
		Timeout:   cfg.RequestTimeout,
	}
}

// NewRetryable returns a retrying client for SDKs that take *retryablehttp.Client.
//
// The retry policy is CheckRetry: 429 and 503-with-Retry-After are retried for any method;
// other 5xx, 408 and transport failures only for idempotent methods, so a POST that
// actually succeeded server-side is never replayed into a duplicate. Backoff is exponential
// (1s to 30s) and honours Retry-After. After the retry budget the last response is handed
// back to the SDK (retryablehttp.PassthroughErrorHandler) so its body can still be
// classified, instead of a nil response and a "giving up" error.
func NewRetryable(cfg Config) *retryablehttp.Client {
	rc := retryablehttp.NewClient()
	rc.HTTPClient = NewPlain(cfg)
	rc.RetryMax = cfg.MaxRetries
	rc.RetryWaitMin = retryWaitMin
	rc.RetryWaitMax = retryWaitMax
	rc.Backoff = retryablehttp.DefaultBackoff
	rc.CheckRetry = CheckRetry
	rc.ErrorHandler = retryablehttp.PassthroughErrorHandler
	rc.Logger = nil
	return rc
}

// NewStandard returns an http.Client backed by NewRetryable, for SDKs that take *http.Client.
func NewStandard(cfg Config) *http.Client {
	return NewRetryable(cfg).StandardClient()
}

// IdempotentMethod reports whether a request with this method can be replayed safely.
func IdempotentMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodDelete:
		return true
	}
	return false
}

var (
	redirectsErrorRe  = regexp.MustCompile(`stopped after \d+ redirects\z`)
	schemeErrorRe     = regexp.MustCompile(`unsupported protocol scheme`)
	notTrustedErrorRe = regexp.MustCompile(`certificate is not trusted`)
)

// CheckRetry is the retryablehttp.CheckRetry policy described on NewRetryable.
func CheckRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		var urlErr *url.Error
		if !errors.As(err, &urlErr) {
			return false, nil
		}
		msg := urlErr.Error()
		if redirectsErrorRe.MatchString(msg) || schemeErrorRe.MatchString(msg) || notTrustedErrorRe.MatchString(msg) {
			return false, nil
		}
		var unknownAuthority x509.UnknownAuthorityError
		if errors.As(err, &unknownAuthority) {
			return false, nil
		}
		// url.Error.Op is the request method ("Get", "Post", ...).
		return IdempotentMethod(urlErr.Op), nil
	}
	if resp == nil {
		return false, nil
	}
	method := ""
	if resp.Request != nil {
		method = resp.Request.Method
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return true, nil
	case resp.StatusCode == http.StatusServiceUnavailable && resp.Header.Get("Retry-After") != "":
		// The server states it did not process the request.
		return true, nil
	case resp.StatusCode == http.StatusRequestTimeout, resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented:
		return IdempotentMethod(method), nil
	}
	return false, nil
}
