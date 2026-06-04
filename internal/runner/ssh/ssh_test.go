package ssh

import (
	"strings"
	"testing"

	"github.com/sirrobot01/archbench/spec"
)

func TestSSHArgs(t *testing.T) {
	t.Setenv(insecureEnv, "")

	r := New(spec.Target{
		Host:      "bench-box",
		User:      "ubuntu",
		Port:      2222,
		Key:       "/keys/id_ed25519",
		ProxyJump: "bastion",
	}, "", spec.Cache{})

	args := strings.Join(r.sshArgs(), " ")
	for _, want := range []string{
		"StrictHostKeyChecking=accept-new",
		"BatchMode=yes",
		"-p 2222",
		"-i /keys/id_ed25519",
		"-J bastion",
		"ubuntu@bench-box",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("args %q missing %q", args, want)
		}
	}
}

func TestSSHArgsMinimal(t *testing.T) {
	t.Setenv(insecureEnv, "")

	// With only a host (an ssh_config alias), nothing extra is forced so the
	// user's config resolves user/port/identity.
	r := New(spec.Target{Host: "alias"}, "", spec.Cache{})
	args := r.sshArgs()
	joined := strings.Join(args, " ")

	if args[len(args)-1] != "alias" {
		t.Errorf("destination = %q, want %q", args[len(args)-1], "alias")
	}
	for _, unwanted := range []string{"-p ", "-i ", "-J ", "@"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("args %q should not contain %q", joined, unwanted)
		}
	}
}

func TestSSHArgsInsecure(t *testing.T) {
	t.Setenv(insecureEnv, "1")

	args := strings.Join(New(spec.Target{Host: "h"}, "", spec.Cache{}).sshArgs(), " ")
	if !strings.Contains(args, "StrictHostKeyChecking=no") || !strings.Contains(args, "UserKnownHostsFile=/dev/null") {
		t.Errorf("insecure args missing override: %q", args)
	}
}

func TestInWorkdir(t *testing.T) {
	r := New(spec.Target{Host: "h"}, "", spec.Cache{})
	r.workdir = "/tmp/wd"
	r.cacheDir = "/tmp/wd/.cache"

	// Without custom env, no file is sourced and only the cache path is inline.
	got := r.inWorkdir("go test")
	if strings.Contains(got, ".archbench-env") {
		t.Errorf("did not expect env file sourcing: %q", got)
	}
	if !strings.Contains(got, "cd '/tmp/wd' && go test") {
		t.Errorf("missing workdir cd: %q", got)
	}

	// With env present, the secret file is sourced rather than inlined.
	r.envSourced = true
	got = r.inWorkdir("go test")
	if !strings.Contains(got, ". '/tmp/wd/.archbench-env';") {
		t.Errorf("expected env file to be sourced: %q", got)
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
