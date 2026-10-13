package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resource.UnitTest runs without TF_ACC, so it must only be used for tests that are
// genuinely offline: ones that drive the provider against internal/fakeharness (directly or
// through acctest.RunLifecycle). Acceptance tests that reach the real Harness API use
// resource.Test, which skips unless TF_ACC is set.
func TestUnitTestIsOnlyUsedWithTheFakeAPI(t *testing.T) {
	root := filepath.Join("..", "..")
	var offenders []string
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(src)
		if !strings.Contains(text, "resource.UnitTest(") {
			return nil
		}
		if strings.Contains(text, "fakeharness") || strings.Contains(text, "acctest.RunLifecycle") || strings.Contains(text, "acctest.OfflineFactories") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		offenders = append(offenders, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range offenders {
		t.Errorf("%s calls resource.UnitTest without the fake API; use resource.Test for acceptance tests (it skips without TF_ACC) or point the test at internal/fakeharness", f)
	}
}
