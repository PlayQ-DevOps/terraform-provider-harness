package experiment

// White-box tests for the tag-preservation logic that prevents drift when the
// chaos API adds system-generated tags (e.g. "fault=<identity>") that are not
// part of the user's configuration. Deterministic; no live API / TF_ACC.

import (
	"testing"
)

func TestFlattenTags(t *testing.T) {
	out := flattenTags([]string{"a", "b", "c"})
	if out.Len() != 3 {
		t.Fatalf("expected 3 tags, got %d", out.Len())
	}
	for _, want := range []string{"a", "b", "c"} {
		if !out.Contains(want) {
			t.Errorf("missing tag %q", want)
		}
	}
}
