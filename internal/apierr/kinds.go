// Package apierr classifies errors returned by the Harness SDK clients into a small set of
// kinds that the resource lifecycle helpers act on (not found, conflict, transient, ...).
//
// Every Harness SDK package (nextgen, the OpenAPI nextgen client, har, idp, code,
// svcdiscovery, chaos, dbops, policymgmt, split, cd) has its own error type and its own
// error-body envelope, and several of them panic or lose the message when inspected the
// obvious way. Classify looks only at the HTTP status, the raw body and the error text, so
// it works for every client and never panics.
package apierr

import "fmt"

// Kind is the coarse classification of an API error.
type Kind uint8

const (
	// None means there was no error.
	None Kind = iota
	// Unknown is an error that could not be classified more precisely.
	Unknown
	// NotFound means the object (or its scope) does not exist. On a read this means the
	// resource is gone from Harness and must be removed from state.
	NotFound
	// Unauthorized means the credentials were rejected (HTTP 401).
	Unauthorized
	// Forbidden means the credentials lack permission (HTTP 403).
	Forbidden
	// BadRequest covers validation failures and other client errors (4xx not listed above).
	BadRequest
	// Conflict means the object already exists or the request conflicts with current state.
	Conflict
	// Transient covers rate limiting, server errors and transport failures that may succeed
	// on retry.
	Transient
)

func (k Kind) String() string {
	switch k {
	case None:
		return "none"
	case Unknown:
		return "unknown"
	case NotFound:
		return "not-found"
	case Unauthorized:
		return "unauthorized"
	case Forbidden:
		return "forbidden"
	case BadRequest:
		return "bad-request"
	case Conflict:
		return "conflict"
	case Transient:
		return "transient"
	default:
		return fmt.Sprintf("kind(%d)", uint8(k))
	}
}

// Classified is the result of Classify. It implements error.
type Classified struct {
	Kind Kind
	// Status is the HTTP status code, or 0 when no response was received (the status may
	// still have been recovered from the error text, see Classify).
	Status int
	// Code is the API's own error code when the body carried one: a Harness envelope code
	// such as "ENTITY_NOT_FOUND", a Goa error name such as "not_found", or a gRPC code name
	// such as "NotFound" for GitOps gateway bodies.
	Code string
	// Message is the best human-readable message; never empty when Kind != None.
	Message string
	// Body is the raw response body when the SDK exposed it (at most 64 KiB), else nil.
	Body []byte
	// Transport is true when the request failed before any response was received.
	Transport bool
	// NonJSON is true when a body was present but was not JSON (an HTML error page from a
	// proxy, for example), or when the SDK reported an "undefined response type".
	NonJSON bool
	// Err is the original error.
	Err error
}

// Error returns the message with the HTTP status and API code, when known.
func (c Classified) Error() string {
	switch {
	case c.Kind == None:
		return ""
	case c.Status == 0 && c.Code == "":
		return c.Message
	case c.Code == "":
		return fmt.Sprintf("%s (HTTP %d)", c.Message, c.Status)
	case c.Status == 0:
		return fmt.Sprintf("%s (code %s)", c.Message, c.Code)
	default:
		return fmt.Sprintf("%s (HTTP %d, code %s)", c.Message, c.Status, c.Code)
	}
}

// Unwrap returns the original error so errors.Is/As keep working.
func (c Classified) Unwrap() error { return c.Err }
