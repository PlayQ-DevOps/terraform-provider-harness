package httpclient

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// maxDump bounds how much of a body is logged.
const maxDump = 32 << 10

type loggingTransport struct {
	base      http.RoundTripper
	userAgent string
	secrets   []string
	dump      bool
}

// NewLoggingTransport wraps base so every request is logged through tflog at DEBUG with the
// request-scoped Terraform logger (resource type and request ID come for free). Headers that
// carry credentials, JSON fields whose names say they hold secrets, and the literal secrets
// are redacted. Bodies are only read and logged when TF_LOG or TF_LOG_PROVIDER is DEBUG or
// TRACE, so the common case costs one log line.
func NewLoggingTransport(base http.RoundTripper, userAgent string, secrets []string) http.RoundTripper {
	return &loggingTransport{base: base, userAgent: userAgent, secrets: secrets, dump: bodyDumpEnabled()}
}

func bodyDumpEnabled() bool {
	for _, v := range []string{os.Getenv("TF_LOG_PROVIDER"), os.Getenv("TF_LOG")} {
		switch strings.ToUpper(strings.TrimSpace(v)) {
		case "DEBUG", "TRACE", "JSON":
			return true
		case "":
			continue
		default:
			return false
		}
	}
	return false
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	if t.userAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", t.userAgent)
	}

	var reqBody string
	if t.dump && req.Body != nil && req.Body != http.NoBody {
		raw, err := io.ReadAll(io.LimitReader(req.Body, maxDump+1))
		_ = req.Body.Close()
		if err == nil {
			req.Body = io.NopCloser(bytes.NewReader(raw))
			reqBody = Redact(string(raw), t.secrets)
		}
	}
	tflog.Debug(ctx, "Harness API request", map[string]any{
		"method": req.Method, "url": RedactURL(req.URL.String(), t.secrets), "headers": RedactHeaders(req.Header, t.secrets), "body": reqBody,
	})

	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		tflog.Debug(ctx, "Harness API request failed", map[string]any{"method": req.Method, "url": RedactURL(req.URL.String(), t.secrets), "error": Redact(err.Error(), t.secrets), "duration_ms": time.Since(start).Milliseconds()})
		return nil, err
	}

	var respBody string
	if t.dump && resp.Body != nil {
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxDump+1))
		_ = resp.Body.Close()
		if readErr == nil {
			resp.Body = io.NopCloser(bytes.NewReader(raw))
			respBody = Redact(string(raw), t.secrets)
		}
	}
	tflog.Debug(ctx, "Harness API response", map[string]any{
		"method": req.Method, "url": RedactURL(req.URL.String(), t.secrets), "status": resp.StatusCode, "duration_ms": time.Since(start).Milliseconds(), "body": respBody,
	})
	return resp, nil
}
