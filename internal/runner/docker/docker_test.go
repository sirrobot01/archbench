package docker

import (
	"runtime"
	"strings"
	"testing"

	"github.com/sirrobot01/archbench/spec"
)

func TestInWorkdir(t *testing.T) {
	r := New(spec.Target{Image: "golang:1.24"}, "", spec.Cache{})
	r.cacheDir = "/archbench-cache"

	// Without custom env, no file is sourced and only the cache path is inline.
	got := r.inWorkdir("go test")
	if strings.Contains(got, ".archbench-env") {
		t.Errorf("did not expect env file sourcing: %q", got)
	}
	if !strings.Contains(got, "export ARCHBENCH_CACHE='/archbench-cache'") {
		t.Errorf("missing cache export: %q", got)
	}
	if !strings.Contains(got, "cd '/archbench' && go test") {
		t.Errorf("missing workdir cd: %q", got)
	}

	// With env present, the secret file is sourced rather than inlined.
	r.envSourced = true
	got = r.inWorkdir("go test")
	if !strings.Contains(got, ". '/archbench/.archbench-env';") {
		t.Errorf("expected env file to be sourced: %q", got)
	}
}

func TestCapabilities(t *testing.T) {
	caps := New(spec.Target{Image: "alpine"}, "", spec.Cache{}).Capabilities()
	if caps.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want host arch %q", caps.Arch, runtime.GOARCH)
	}
	if !caps.SupportsPlatform {
		t.Error("expected SupportsPlatform to be true")
	}
}

// TestHostCacheDirScopedByPlatform confirms emulated and native caches don't
// collide: pinning a platform must yield a distinct host cache directory.
func TestHostCacheDirScopedByPlatform(t *testing.T) {
	native := New(spec.Target{Image: "golang"}, "", spec.Cache{Enabled: true, Suite: "svc"})
	emulated := New(spec.Target{Image: "golang", Platform: "linux/amd64"}, "", spec.Cache{Enabled: true, Suite: "svc"})

	nativeDir := native.hostCacheDir()
	emulatedDir := emulated.hostCacheDir()

	if nativeDir == emulatedDir {
		t.Errorf("native and emulated caches share a directory: %q", nativeDir)
	}
	for _, dir := range []string{nativeDir, emulatedDir} {
		if !strings.Contains(dir, "svc") {
			t.Errorf("cache dir %q not scoped by suite", dir)
		}
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"plain":      "'plain'",
		"with space": "'with space'",
		"it's":       `'it'\''s'`,
		"; rm -rf /": `'; rm -rf /'`,
		"$(reboot)":  "'$(reboot)'",
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
