package apierr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"google.golang.org/grpc/codes"
)

// maxBody bounds how much of a response body Classify retains.
const maxBody = 64 << 10

// The generated SDK error types all expose the raw body and the decoded model through
// these methods. They are asserted through errors.As so wrapped errors still classify.
// GenericSwaggerError.Code() is deliberately never called: in the nextgen SDK it type-asserts
// the decoded model and panics for almost every 404.
type bodyer interface{ Body() []byte }

var (
	// "404 Not Found" (the raw status text the generated SDKs use as the error string).
	statusPrefixRe = regexp.MustCompile(`^(\d{3}) [A-Za-z]`)
	// "unexpected status code: 503, body: ..." (chaos GraphQL transport).
	unexpectedStatusRe = regexp.MustCompile(`unexpected status code: (\d{3})`)
	// "environments delete: 404 404 Not Found: {...}" (split SDK: "<op>: %d %s: %s").
	opStatusRe = regexp.MustCompile(`^[a-z][a-z0-9 _-]*: (\d{3}) (?:\d{3} )?[A-Za-z]`)
	// retryablehttp's default ErrorHandler after the retry budget is exhausted.
	givingUpRe = regexp.MustCompile(`giving up after \d+ attempt`)
	// Argo CD errors proxied as HTTP 500 by the GitOps gateway.
	grpcNotFoundRe   = regexp.MustCompile(`rpc error: code = NotFound`)
	alreadyExistsRe  = regexp.MustCompile(`(?i)\balready exists\b|\bduplicate key\b`)
	graphqlPrefixRe  = regexp.MustCompile(`^graphql error: `)
	graphqlPathRe    = regexp.MustCompile(` \(path: .*\)$`)
	htmlTagRe        = regexp.MustCompile(`<[^>]*>`)
	whitespaceRunRe  = regexp.MustCompile(`\s+`)
	rawStatusTextRe  = regexp.MustCompile(`^\d{3} [A-Za-z' -]+$`)
	notFoundCodes    = map[string]bool{"ENTITY_NOT_FOUND": true, "RESOURCE_NOT_FOUND_EXCEPTION": true, "RESOURCE_NOT_FOUND": true, "NOTFOUND": true, "NOT_FOUND": true}
	conflictCodes    = map[string]bool{"DUPLICATE_FIELD": true, "ALREADYEXISTS": true, "ALREADY_EXISTS": true}
	forbiddenCodes   = map[string]bool{"PERMISSIONDENIED": true, "PERMISSION_DENIED": true}
	unauthCodes      = map[string]bool{"UNAUTHENTICATED": true}
	badRequestCodes  = map[string]bool{"INVALIDARGUMENT": true, "INVALID_ARGUMENT": true, "FAILEDPRECONDITION": true, "FAILED_PRECONDITION": true, "OUTOFRANGE": true, "OUT_OF_RANGE": true}
	transientCodes   = map[string]bool{"UNAVAILABLE": true, "DEADLINEEXCEEDED": true, "DEADLINE_EXCEEDED": true, "RESOURCEEXHAUSTED": true, "RESOURCE_EXHAUSTED": true, "ABORTED": true}
	conflictStatuses = map[int]bool{0: true, 400: true, 409: true, 500: true}
)

// StatusOf returns resp's status code, or 0 for a nil response.
func StatusOf(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

// IsNotFound reports whether err/resp classify as NotFound.
func IsNotFound(err error, resp *http.Response) bool {
	return Classify(err, resp).Kind == NotFound
}

// IsTransport reports whether the request failed before any response was received.
func IsTransport(err error, resp *http.Response) bool {
	return Classify(err, resp).Transport
}

// Classify inspects an SDK error and the HTTP response (which may be nil) and returns a
// Classified error. It never panics. The HTTP status decides the Kind first; the body's
// error code can only make the classification more specific (for example a 400 or 500
// whose body says ENTITY_NOT_FOUND is NotFound).
func Classify(err error, resp *http.Response) Classified {
	if err == nil {
		return Classified{Kind: None}
	}
	c := Classified{Err: err, Status: StatusOf(resp)}
	text := err.Error()

	var b bodyer
	if errors.As(err, &b) {
		body := b.Body()
		if len(body) > maxBody {
			body = body[:maxBody]
		}
		c.Body = body
	}

	if c.Status == 0 {
		c.Status = statusFromText(text)
	}

	if resp == nil && c.Status == 0 && c.Body == nil {
		if errors.Is(err, context.Canceled) {
			c.Kind = Unknown
			c.Message = "operation cancelled before the Harness API responded: " + text
			return c
		}
		// A client timeout unwraps to context.DeadlineExceeded, so transport failures are
		// recognised before deadlines; both are transient.
		if isTransportError(err) || errors.Is(err, context.DeadlineExceeded) {
			c.Kind = Transient
			c.Transport = true
			c.Message = transportMessage(err)
			return c
		}
		if givingUpRe.MatchString(text) {
			c.Kind = Transient
			c.Message = "the Harness API kept failing until the retry budget was exhausted: " + text
			return c
		}
	}

	env, parsed := parseEnvelope(c.Body)
	if !parsed && len(bytes.TrimSpace(c.Body)) > 0 {
		c.NonJSON = true
	}
	if strings.Contains(text, "undefined response type") {
		c.NonJSON = true
	}

	c.Code = env.code()
	c.Message = bestMessage(text, c, env)
	c.Kind = kindForStatus(c.Status)
	refine(&c, env)
	return c
}

func isTransportError(err error) bool {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	return false
}

func transportMessage(err error) string {
	var netErr net.Error
	timedOut := errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout())
	var urlErr *url.Error
	switch {
	case timedOut && errors.As(err, &urlErr):
		return fmt.Sprintf("timed out waiting for the Harness API (%s %s): %v", urlErr.Op, urlErr.URL, urlErr.Err)
	case timedOut:
		return "timed out waiting for the Harness API: " + err.Error()
	case errors.As(err, &urlErr):
		return fmt.Sprintf("could not reach the Harness API (%s %s): %v", urlErr.Op, urlErr.URL, urlErr.Err)
	default:
		return "could not reach the Harness API: " + err.Error()
	}
}

func statusFromText(text string) int {
	for _, re := range []*regexp.Regexp{statusPrefixRe, unexpectedStatusRe, opStatusRe} {
		if m := re.FindStringSubmatch(text); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				return n
			}
		}
	}
	return 0
}

// envelope is a loose view of the JSON error bodies the Harness services return. Fields
// that different services type differently are kept raw.
type envelope struct {
	Status           string            `json:"status"`
	Code             json.RawMessage   `json:"code"`
	Message          string            `json:"message"`
	Error            json.RawMessage   `json:"error"`
	Description      string            `json:"description"`
	DetailedMessage  string            `json:"detailedMessage"`
	Name             string            `json:"name"`
	Temporary        bool              `json:"temporary"`
	Timeout          bool              `json:"timeout"`
	ResponseMessages []responseMessage `json:"responseMessages"`
	Errors           []json.RawMessage `json:"errors"`
}

type responseMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func parseEnvelope(body []byte) (envelope, bool) {
	var e envelope
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return envelope{}, false
	}
	if err := json.Unmarshal(trimmed, &e); err != nil {
		return envelope{}, false
	}
	return e, true
}

// code returns the envelope's error code: a string code as-is, a small number as the gRPC
// code name (GitOps gateway bodies), another number as digits, else the Goa error name.
func (e envelope) code() string {
	if len(e.Code) > 0 {
		var s string
		if err := json.Unmarshal(e.Code, &s); err == nil {
			return s
		}
		var n int
		if err := json.Unmarshal(e.Code, &n); err == nil {
			if n >= 0 && n <= int(codes.Unauthenticated) {
				return codes.Code(n).String()
			}
			return strconv.Itoa(n)
		}
	}
	return e.Name
}

func (e envelope) errorString() string {
	if len(e.Error) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(e.Error, &s); err == nil {
		return s
	}
	var m struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(e.Error, &m); err == nil {
		return m.Message
	}
	return ""
}

func (e envelope) firstError() string {
	for _, raw := range e.Errors {
		var v struct {
			FieldID string `json:"fieldId"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			continue
		}
		switch {
		case v.FieldID != "" && v.Error != "":
			return v.FieldID + " " + v.Error
		case v.Error != "":
			return v.Error
		case v.Message != "":
			return v.Message
		}
	}
	return ""
}

func bestMessage(text string, c Classified, env envelope) string {
	msg := ""
	switch {
	case env.Message != "":
		msg = env.Message
		if env.Description != "" && env.Description != env.Message {
			msg += ": " + env.Description
		}
	case len(env.ResponseMessages) > 0 && env.ResponseMessages[0].Message != "":
		msg = env.ResponseMessages[0].Message
	case env.firstError() != "":
		msg = env.firstError()
	case env.Description != "":
		msg = env.Description
	case env.DetailedMessage != "":
		msg = env.DetailedMessage
	case env.errorString() != "":
		msg = env.errorString()
	}
	if msg == "" && !rawStatusTextRe.MatchString(text) && !strings.Contains(text, "undefined response type") {
		msg = cleanGraphQL(text)
	}
	if msg == "" && c.NonJSON {
		snippet := strings.TrimSpace(whitespaceRunRe.ReplaceAllString(htmlTagRe.ReplaceAllString(string(c.Body), " "), " "))
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		msg = statusText(c.Status, text) + " (non-JSON response body)"
		if snippet != "" {
			msg += ": " + snippet
		}
	}
	if msg == "" {
		msg = statusText(c.Status, text)
	}
	return strings.TrimSpace(msg)
}

func statusText(status int, fallback string) string {
	if status != 0 {
		if t := http.StatusText(status); t != "" {
			return t
		}
	}
	return fallback
}

func cleanGraphQL(text string) string {
	if !graphqlPrefixRe.MatchString(text) {
		return text
	}
	text = graphqlPrefixRe.ReplaceAllString(text, "")
	text = graphqlPathRe.ReplaceAllString(text, "")
	return text
}

func kindForStatus(status int) Kind {
	switch {
	case status == 0:
		return Unknown
	case status == http.StatusUnauthorized:
		return Unauthorized
	case status == http.StatusForbidden:
		return Forbidden
	case status == http.StatusNotFound || status == http.StatusGone:
		return NotFound
	case status == http.StatusConflict:
		return Conflict
	case status == http.StatusRequestTimeout || status == http.StatusTooEarly || status == http.StatusTooManyRequests:
		return Transient
	case status == http.StatusNotImplemented:
		return Unknown
	case status >= 500:
		return Transient
	case status >= 400:
		return BadRequest
	default:
		// A 2xx/3xx with an error means the SDK could not decode the response.
		return Unknown
	}
}

// refine makes the status-based Kind more specific using the body. It never demotes a
// NotFound and never overrides an authentication failure.
func refine(c *Classified, env envelope) {
	if c.Kind == Unauthorized || c.Kind == Forbidden {
		return
	}
	code := strings.ToUpper(c.Code)
	if notFoundCodes[code] || grpcNotFoundRe.MatchString(c.Message) {
		c.Kind = NotFound
		return
	}
	if c.Kind == NotFound {
		return
	}
	switch {
	case unauthCodes[code] || c.Message == "unauthorized":
		c.Kind = Unauthorized
	case forbiddenCodes[code]:
		c.Kind = Forbidden
	case conflictCodes[code] && conflictStatuses[c.Status]:
		c.Kind = Conflict
	case alreadyExistsRe.MatchString(c.Message) && conflictStatuses[c.Status]:
		c.Kind = Conflict
	case badRequestCodes[code]:
		c.Kind = BadRequest
	case transientCodes[code]:
		c.Kind = Transient
	case (env.Temporary || env.Timeout) && c.Status >= 500:
		c.Kind = Transient
	}
}
