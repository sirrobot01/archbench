//go:build integration

// Layer 3 integration test: the runner exercised against a REAL docker daemon,
// catching tar/exec/tooling interop the unit tests can't.
//
// Run it explicitly:
//
//	go test -tags integration ./internal/runner/docker/
//
// Skipped when docker is unavailable.
package docker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirrobot01/archbench"
)

// image is small, ships a POSIX shell plus tar, and exists for both amd64 and
// arm64, so the test stays fast and arch-agnostic.
const image = "alpine:3.20"

func TestIntegrationRealDocker(t *testing.T) {
	requireDocker(t)

	project := t.TempDir()
	writeFile(t, project, "hello.txt", "real docker\n")
	writeFile(t, project, "nested/data.txt", "nested payload\n")

	r := New(archbench.Target{Type: archbench.TargetDocker, Image: image}, project, archbench.Cache{})

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if err := r.Prepare(ctx); err != nil {
		t.Fatalf("Prepare against real docker: %v", err)
	}
	container := r.container
	t.Cleanup(func() { _ = r.Cleanup(ctx) })

	out, err := r.Execute(ctx, archbench.Run{
		Setup:   []string{"test -f hello.txt", "test -f nested/data.txt"},
		Command: `cat hello.txt nested/data.txt; echo "secret=$TOKEN"; echo "cache=$ARCHBENCH_CACHE"`,
		Env:     map[string]string{"TOKEN": "s3cr3t"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Stdout, "real docker") || !strings.Contains(out.Stdout, "nested payload") {
		t.Errorf("uploaded files not found in workdir; stdout=%q stderr=%q", out.Stdout, out.Stderr)
	}
	// The env value (passed via the sourced file, not argv) reaches the command.
	if !strings.Contains(out.Stdout, "secret=s3cr3t") {
		t.Errorf("env from sourced file not applied; stdout=%q", out.Stdout)
	}
	if !strings.Contains(out.Stdout, "cache=/archbench/.cache") {
		t.Errorf("cache variable not set to ephemeral dir; stdout=%q", out.Stdout)
	}
	if out.OS != "linux" || out.Arch == "" {
		t.Errorf("expected linux OS and an Arch, got OS=%q Arch=%q", out.OS, out.Arch)
	}

	// A non-zero command exit is a result, not a runner error.
	fail, err := r.Execute(ctx, archbench.Run{Command: "echo boom; exit 3"})
	if err != nil {
		t.Fatalf("non-zero exit should not be a runner error: %v", err)
	}
	if fail.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", fail.ExitCode)
	}

	if err := r.Cleanup(ctx); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if containerExists(t, container) {
		t.Errorf("container %q still present after Cleanup", container)
	}
}

func requireDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not installed")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not available")
	}
}

func containerExists(t *testing.T, id string) bool {
	t.Helper()
	out, err := exec.Command("docker", "ps", "-aq", "--no-trunc", "--filter", "id="+id).Output()
	if err != nil {
		t.Fatalf("docker ps: %v", err)
	}
	return strings.TrimSpace(string(out)) != ""
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
