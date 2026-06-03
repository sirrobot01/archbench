package ssh

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// nopWriteCloser lets us hand streamTar a buffer it can Close.
type nopWriteCloser struct{ *bytes.Buffer }

func (nopWriteCloser) Close() error { return nil }

func TestStreamTar(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module demo\n")
	writeFile(t, root, "pkg/calc.go", "package pkg\n")
	writeFile(t, root, ".git/config", "secret")              // excluded
	writeFile(t, root, "archbench-results/local.json", "{}") // excluded

	if runtime.GOOS != "windows" {
		if err := os.Symlink("go.mod", filepath.Join(root, "link")); err != nil {
			t.Fatalf("symlink: %v", err)
		}
	}

	var buf bytes.Buffer
	if err := streamTar(nopWriteCloser{&buf}, root); err != nil {
		t.Fatalf("streamTar: %v", err)
	}

	got := untar(t, &buf)

	if got["go.mod"] != "module demo\n" {
		t.Errorf("go.mod content = %q", got["go.mod"])
	}
	if got["pkg/calc.go"] != "package pkg\n" {
		t.Errorf("pkg/calc.go content = %q", got["pkg/calc.go"])
	}
	for _, excluded := range []string{".git/config", "archbench-results/local.json"} {
		if _, ok := got[excluded]; ok {
			t.Errorf("expected %q to be excluded from archive", excluded)
		}
	}
}

func TestStreamTarRespectsGitIgnore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	root := t.TempDir()
	runGit(t, root, "init")
	writeFile(t, root, ".gitignore", "ignored/\n*.log\n")
	writeFile(t, root, "tracked.txt", "tracked\n")
	writeFile(t, root, "untracked.txt", "untracked\n")
	writeFile(t, root, "ignored/secret.txt", "secret\n")
	writeFile(t, root, "debug.log", "debug\n")
	runGit(t, root, "add", ".gitignore", "tracked.txt")

	var buf bytes.Buffer
	if err := streamTar(nopWriteCloser{&buf}, root); err != nil {
		t.Fatalf("streamTar: %v", err)
	}

	got := untar(t, &buf)
	for path, want := range map[string]string{
		".gitignore":    "ignored/\n*.log\n",
		"tracked.txt":   "tracked\n",
		"untracked.txt": "untracked\n",
	} {
		if got[path] != want {
			t.Errorf("%s content = %q, want %q", path, got[path], want)
		}
	}
	for _, excluded := range []string{".git/config", "ignored/secret.txt", "debug.log"} {
		if _, ok := got[excluded]; ok {
			t.Errorf("expected %q to be excluded from archive", excluded)
		}
	}
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

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// untar returns regular-file contents keyed by slash path.
func untar(t *testing.T, r io.Reader) map[string]string {
	t.Helper()
	gz, err := gzip.NewReader(r)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	tr := tar.NewReader(gz)
	out := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		if hdr.Typeflag == tar.TypeReg {
			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("read %s: %v", hdr.Name, err)
			}
			out[hdr.Name] = string(b)
		}
	}
	return out
}
