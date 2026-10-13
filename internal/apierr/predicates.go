package apierr

import (
	"fmt"
	"strings"
)

// Predicate reports whether a classified error matches some condition. The lifecycle
// helpers accept predicates to treat backend-specific signals as NotFound (a chaos 500
// whose message says "no matching infra", for example) without widening the generic rules.
type Predicate func(Classified) bool

// StatusIs matches any of the given HTTP status codes.
func StatusIs(status ...int) Predicate {
	return func(c Classified) bool {
		for _, s := range status {
			if c.Status == s {
				return true
			}
		}
		return false
	}
}

// CodeIs matches any of the given API error codes, ignoring case.
func CodeIs(code ...string) Predicate {
	return func(c Classified) bool {
		for _, want := range code {
			if strings.EqualFold(c.Code, want) {
				return true
			}
		}
		return false
	}
}

// MinPhraseLength is the shortest phrase MessageHasPhrase accepts. Short phrases such as
// "not found" match far too much (a missing parent, a missing permission), so a phrase
// must name the thing, e.g. "hub not found".
const MinPhraseLength = 8

// MessageHasPhrase matches when the message contains any of the phrases, ignoring case.
// It panics for a phrase shorter than MinPhraseLength; the phrases are package-level
// constants, so this surfaces in the provider's own tests.
func MessageHasPhrase(phrase ...string) Predicate {
	for _, p := range phrase {
		if len(p) < MinPhraseLength {
			panic(fmt.Sprintf("apierr.MessageHasPhrase: phrase %q is shorter than %d characters and would match unrelated errors", p, MinPhraseLength))
		}
	}
	return func(c Classified) bool {
		msg := strings.ToLower(c.Message)
		for _, p := range phrase {
			if strings.Contains(msg, strings.ToLower(p)) {
				return true
			}
		}
		return false
	}
}

// And matches when every predicate matches.
func And(preds ...Predicate) Predicate {
	return func(c Classified) bool {
		for _, p := range preds {
			if !p(c) {
				return false
			}
		}
		return len(preds) > 0
	}
}

// Or matches when any predicate matches.
func Or(preds ...Predicate) Predicate {
	return func(c Classified) bool {
		for _, p := range preds {
			if p(c) {
				return true
			}
		}
		return false
	}
}
