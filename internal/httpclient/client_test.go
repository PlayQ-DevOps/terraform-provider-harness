package httpclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/harness/terraform-provider-harness/internal/apierr"
)

func TestCheckRetry(t *testing.T) {
	ctx := context.Background()
	resp := func(method string, status int, header ...string) *http.Response {
		r := &http.Response{StatusCode: status, Header: http.Header{}, Request: &http.Request{Method: method}}
		for i := 0; i+1 < len(header); i += 2 {
			r.Header.Set(header[i], header[i+1])
		}
		return r
	}
	cases := []struct {
		name  string
		resp  *http.Response
		err   error
		retry bool
	}{
		{"GET 200", resp("GET", 200), nil, false},
		{"GET 404", resp("GET", 404), nil, false},
		{"GET 429", resp("GET", 429), nil, true},
		{"POST 429", resp("POST", 429), nil, true},
		{"POST 503 with Retry-After", resp("POST", 503, "Retry-After", "0"), nil, true},
		{"POST 503 without Retry-After", resp("POST", 503), nil, false},
		{"GET 503 without Retry-After", resp("GET", 503), nil, true},
		{"GET 500", resp("GET", 500), nil, true},
		{"DELETE 500", resp("DELETE", 500), nil, true},
		{"POST 500", resp("POST", 500), nil, false},
		{"PUT 502", resp("PUT", 502), nil, false},
		{"PATCH 504", resp("PATCH", 504), nil, false},
		{"GET 501", resp("GET", 501), nil, false},
		{"GET 408", resp("GET", 408), nil, true},
		{"POST 408", resp("POST", 408), nil, false},
		{"GET connection refused", nil, &url.Error{Op: "Get", URL: "x", Err: errors.New("connection refused")}, true},
		{"POST connection refused", nil, &url.Error{Op: "Post", URL: "x", Err: errors.New("connection refused")}, false},
		{"GET too many redirects", nil, &url.Error{Op: "Get", URL: "x", Err: errors.New("stopped after 10 redirects")}, false},
		{"GET bad scheme", nil, &url.Error{Op: "Get", URL: "x", Err: errors.New("unsupported protocol scheme \"\"")}, false},
		{"GET untrusted cert", nil, &url.Error{Op: "Get", URL: "x", Err: errors.New("x509: certificate is not trusted")}, false},
		{"non-url error", nil, errors.New("boom"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			retry, err := CheckRetry(ctx, tc.resp, tc.err)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if retry != tc.retry {
				t.Errorf("retry = %v, want %v", retry, tc.retry)
			}
		})
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if retry, err := CheckRetry(cancelled, resp("GET", 500), nil); retry || err == nil {
		t.Error("a cancelled context must stop retrying with its error")
	}
}

func countingServer(t *testing.T, statuses ...int) (*httptest.Server, *int32) {
	t.Helper()
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(atomic.AddInt32(&n, 1)) - 1
		status := statuses[len(statuses)-1]
		if i < len(statuses) {
			status = statuses[i]
		}
		if status == 429 || status == 503 {
			w.Header().Set("Retry-After", "0")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"status":"ERROR","message":"m"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

func TestRetryableClientPolicy(t *testing.T) {
	cfg := Config{MaxRetries: 3}

	srv, n := countingServer(t, 500, 500, 200)
	resp, err := NewStandard(cfg).Get(srv.URL)
	if err != nil || resp.StatusCode != 200 || atomic.LoadInt32(n) != 3 {
		t.Fatalf("GET 500,500,200: err=%v status=%d attempts=%d", err, statusOf(resp), atomic.LoadInt32(n))
	}

	srv, n = countingServer(t, 500, 200)
	resp, err = NewStandard(cfg).Post(srv.URL, "application/json", strings.NewReader(`{}`))
	if err != nil || resp.StatusCode != 500 || atomic.LoadInt32(n) != 1 {
		t.Fatalf("POST 500 must not be retried and the response must be passed through: err=%v status=%d attempts=%d", err, statusOf(resp), atomic.LoadInt32(n))
	}

	srv, n = countingServer(t, 429, 200)
	resp, err = NewStandard(cfg).Post(srv.URL, "application/json", strings.NewReader(`{}`))
	if err != nil || resp.StatusCode != 200 || atomic.LoadInt32(n) != 2 {
		t.Fatalf("POST 429 with Retry-After must be retried: err=%v status=%d attempts=%d", err, statusOf(resp), atomic.LoadInt32(n))
	}

	srv, n = countingServer(t, 503)
	resp, err = NewStandard(cfg).Get(srv.URL)
	if err != nil || resp.StatusCode != 503 || atomic.LoadInt32(n) != 4 {
		t.Fatalf("persistent 503: the last response must be passed through after MaxRetries+1 attempts: err=%v status=%d attempts=%d", err, statusOf(resp), atomic.LoadInt32(n))
	}
	if c := apierr.Classify(errors.New("503 Service Unavailable"), resp); c.Kind != apierr.Transient {
		t.Errorf("classify passthrough 503 = %s", c.Kind)
	}
}

func TestRequestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	req, _ := http.NewRequest("POST", srv.URL, nil)
	_, err := NewStandard(Config{MaxRetries: 2, RequestTimeout: 50 * time.Millisecond}).Do(req)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	c := apierr.Classify(err, nil)
	if c.Kind != apierr.Transient || !c.Transport {
		t.Errorf("timeout classified as %+v", c)
	}
}

func TestUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = r.Header.Get("User-Agent") }))
	t.Cleanup(srv.Close)
	if _, err := NewPlain(Config{UserAgent: "terraform-provider-harness-test"}).Get(srv.URL); err != nil {
		t.Fatal(err)
	}
	if got != "terraform-provider-harness-test" {
		t.Errorf("User-Agent = %q", got)
	}
}

func statusOf(r *http.Response) int {
	if r == nil {
		return 0
	}
	return r.StatusCode
}
