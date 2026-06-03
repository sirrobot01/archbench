package validate

import (
	"runtime"
	"testing"
)

func TestNonEmptyAccepts(t *testing.T) {
	if err := NonEmpty("archbench"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNonEmptyRejects(t *testing.T) {
	if err := NonEmpty(""); err == nil {
		t.Fatal("expected an error for an empty name")
	}
}

// TestArchSpecific skips on every architecture except amd64, so its status
// diverges between, say, an arm64 laptop (skip) and an amd64 host (pass). Run
// the suite on two targets and `archbench compare` flags it as DIVERGES -- the
// thing test mode is for.
func TestArchSpecific(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skipf("only validated on amd64, not %s", runtime.GOARCH)
	}
	if err := NonEmpty("amd64"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
