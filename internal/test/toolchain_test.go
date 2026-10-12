package test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The Go version is pinned once, in go.mod's `toolchain` directive. .tool-versions
// mirrors it for asdf/CI convenience; this test keeps the two from drifting.
func TestToolVersionsMatchGoModToolchain(t *testing.T) {
	root := filepath.Join("..", "..")

	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	toolchain := regexp.MustCompile(`(?m)^toolchain go(\S+)`).FindSubmatch(goMod)
	if toolchain == nil {
		t.Fatalf("go.mod has no `toolchain goX.Y.Z` directive; it is the single source of truth for the Go version")
	}

	toolVersions, err := os.ReadFile(filepath.Join(root, ".tool-versions"))
	if err != nil {
		t.Fatalf("read .tool-versions: %v", err)
	}
	golang := regexp.MustCompile(`(?m)^golang (\S+)`).FindSubmatch(toolVersions)
	if golang == nil {
		t.Fatalf(".tool-versions has no `golang` line")
	}

	if got, want := string(golang[1]), string(toolchain[1]); got != want {
		t.Fatalf(".tool-versions pins golang %s but go.mod's toolchain directive is go%s; update .tool-versions to match go.mod", got, want)
	}
}
