//go:build integration

// Layer 3 integration test: the runner exercised against a REAL OpenSSH daemon
// through the system ssh client, catching protocol/tar/tooling interop the unit
// tests can't.
//
// Run it explicitly:
//
//	go test -tags integration ./internal/runner/ssh/
//
// Backends, in priority order:
//   - ARCHBENCH_SSH_TEST_HOST set -> run against that host (with _USER, _KEY).
//   - else Docker available       -> auto-provision an openssh-server container.
//   - else                        -> skipped.
package ssh

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sirrobot01/archbench"
)

const sshImage = "lscr.io/linuxserver/openssh-server:latest"

func TestIntegrationRealOpenSSH(t *testing.T) {
	requireSSHTools(t)
	target := provision(t)

	project := t.TempDir()
	writeFile(t, project, "hello.txt", "real openssh\n")
	writeFile(t, project, "nested/data.txt", "nested payload\n")

	r := New(target, project, archbench.Cache{})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := r.Prepare(ctx); err != nil {
		t.Fatalf("Prepare against real sshd: %v", err)
	}
	t.Cleanup(func() { _ = r.Cleanup(ctx) })

	out, err := r.Execute(ctx, archbench.Run{
		Setup:   []string{"test -f hello.txt", "test -f nested/data.txt"},
		Command: `cat hello.txt nested/data.txt; echo "secret=$TOKEN"`,
		Env:     map[string]string{"TOKEN": "s3cr3t"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Stdout, "real openssh") || !strings.Contains(out.Stdout, "nested payload") {
		t.Errorf("uploaded files not found in synced workdir; stdout=%q stderr=%q", out.Stdout, out.Stderr)
	}
	// The env value (passed via the sourced file, not argv) reaches the command.
	if !strings.Contains(out.Stdout, "secret=s3cr3t") {
		t.Errorf("env from sourced file not applied; stdout=%q", out.Stdout)
	}
	if out.OS == "" || out.Arch == "" {
		t.Errorf("expected OS/Arch metadata, got OS=%q Arch=%q", out.OS, out.Arch)
	}

	workdir := r.workdir
	if err := r.Cleanup(ctx); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if remoteExists(t, target, workdir) {
		t.Errorf("workdir %q still present after Cleanup", workdir)
	}
}

func requireSSHTools(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ssh", "ssh-keygen"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("missing required tool %q", bin)
		}
	}
}

// provision returns a target for a real sshd, choosing a backend.
func provision(t *testing.T) archbench.Target {
	if host := os.Getenv("ARCHBENCH_SSH_TEST_HOST"); host != "" {
		user, key := os.Getenv("ARCHBENCH_SSH_TEST_USER"), os.Getenv("ARCHBENCH_SSH_TEST_KEY")
		if user == "" || key == "" {
			t.Fatal("ARCHBENCH_SSH_TEST_HOST set but _USER/_KEY missing")
		}
		return archbench.Target{Type: archbench.TargetSSH, Host: host, Port: envPort(t), User: user, Key: key}
	}
	if !dockerAvailable() {
		t.Skip("no ARCHBENCH_SSH_TEST_HOST and docker unavailable; skipping real-openssh test")
	}
	return provisionDocker(t)
}

func envPort(t *testing.T) int {
	p := os.Getenv("ARCHBENCH_SSH_TEST_PORT")
	if p == "" {
		return 0
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		t.Fatalf("bad ARCHBENCH_SSH_TEST_PORT %q: %v", p, err)
	}
	return n
}

func dockerAvailable() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	return exec.Command("docker", "info").Run() == nil
}

func provisionDocker(t *testing.T) archbench.Target {
	// The container's host key is ephemeral, so skip verification for it.
	t.Setenv(insecureEnv, "1")

	keyPath, pubLine := genKeyPair(t)
	const user = "tester"

	out, err := exec.Command("docker", "run", "-d", "--rm",
		"-e", "PUID=1000", "-e", "PGID=1000",
		"-e", "USER_NAME="+user,
		"-e", "PUBLIC_KEY="+pubLine,
		"-e", "SUDO_ACCESS=false",
		"-e", "PASSWORD_ACCESS=false",
		"-p", "127.0.0.1::2222",
		sshImage,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("docker run: %v\n%s", err, out)
	}
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", id).Run() })

	portOut, err := exec.Command("docker", "port", id, "2222").Output()
	if err != nil {
		t.Fatalf("docker port: %v", err)
	}
	addr := strings.TrimSpace(strings.SplitN(string(portOut), "\n", 2)[0]) // "127.0.0.1:49xxx"
	host, p, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("parse docker port %q: %v", addr, err)
	}
	port, _ := strconv.Atoi(p)

	waitForSSH(t, host, port, 45*time.Second)
	return archbench.Target{Type: archbench.TargetSSH, Host: host, Port: port, User: user, Key: keyPath}
}

// waitForSSH blocks until the daemon presents an SSH banner or timeout.
func waitForSSH(t *testing.T, host string, port int, timeout time.Duration) {
	t.Helper()
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			buf := make([]byte, 4)
			n, _ := conn.Read(buf)
			_ = conn.Close()
			if n >= 4 && string(buf[:4]) == "SSH-" {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("sshd at %s not ready within %s", addr, timeout)
}

// remoteExists reports whether path exists on the host, via a fresh runner.
func remoteExists(t *testing.T, target archbench.Target, path string) bool {
	t.Helper()
	vr := New(target, "", archbench.Cache{})
	_, _, err := vr.run(context.Background(), "test -e "+shellQuote(path))
	return err == nil
}

// genKeyPair generates an ed25519 keypair with ssh-keygen and returns the
// private key path and the authorized_keys public line.
func genKeyPair(t *testing.T) (privPath, pubLine string) {
	t.Helper()
	privPath = filepath.Join(t.TempDir(), "id_ed25519")
	if out, err := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-q", "-f", privPath).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v\n%s", err, out)
	}
	pub, err := os.ReadFile(privPath + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	return privPath, strings.TrimSpace(string(pub))
}
