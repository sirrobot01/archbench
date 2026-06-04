package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSpecDefaultsAndValidates(t *testing.T) {
	path := writeSpec(t, `
name: demo
targets:
  - name: local-arm64
    type: local
runs:
  - name: all
    command: go test ./... -bench=.
`)

	spec, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	if spec.Mode != ModeBench {
		t.Errorf("mode = %q, want %q", spec.Mode, ModeBench)
	}
	if spec.Parser != "go-test" {
		t.Errorf("parser = %q, want go-test", spec.Parser)
	}
	if len(spec.Runs) != 1 || spec.Runs[0].Name != "all" {
		t.Fatalf("runs = %#v, want one named run", spec.Runs)
	}
}

func TestLoadSpecRuns(t *testing.T) {
	path := writeSpec(t, `
name: demo
targets:
  - name: local-arm64
    type: local
runs:
  - name: parser
    command: go test ./internal/parser/... -bench=.
  - name: stream
    setup:
      - go mod download
    env:
      GOMAXPROCS: "4"
    command: go test ./pkg/stream/... -bench=.
`)

	spec, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	if len(spec.Runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(spec.Runs))
	}
	if spec.Runs[0].Name != "parser" || spec.Runs[1].Name != "stream" {
		t.Fatalf("run names = %#v", spec.Runs)
	}
}

func TestLoadSpecStrictAndTargetValidation(t *testing.T) {
	cases := map[string]string{
		"unknown field": `
name: demo
unknown: nope
targets:
  - name: local
    type: local
runs:
  - name: all
    command: echo ok
`,
		"ssh without host": `
name: demo
targets:
  - name: amd64
    type: ssh
runs:
  - name: all
    command: echo ok
`,
		"duplicate target": `
name: demo
targets:
  - name: local
    type: local
  - name: local
    type: local
runs:
  - name: all
    command: echo ok
`,
		"missing runs": `
name: demo
targets:
  - name: local
    type: local
`,
		"unnamed runs entry": `
name: demo
targets:
  - name: local
    type: local
runs:
  - command: echo ok
`,
		"duplicate run": `
name: demo
targets:
  - name: local
    type: local
runs:
  - name: bench
    command: echo ok
  - name: bench
    command: echo ok
`,
		"exec on non-ssh target": `
name: demo
targets:
  - name: local
    type: local
    exec: true
runs:
  - name: all
    command: echo ok
`,
		"execBinary without exec": `
name: demo
targets:
  - name: amd64
    type: ssh
    host: box
    execBinary: /usr/local/bin/archbench
runs:
  - name: all
    command: echo ok
`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadSpec(writeSpec(t, body)); err == nil {
				t.Fatal("LoadSpec returned nil error")
			}
		})
	}
}

func TestLoadSpecExecTarget(t *testing.T) {
	s, err := LoadSpec(writeSpec(t, `
name: demo
targets:
  - name: amd64
    type: ssh
    host: box
    exec: true
    execBinary: /usr/local/bin/archbench
runs:
  - name: all
    command: go test ./... -bench=.
`))
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	if !s.Targets[0].Exec || s.Targets[0].ExecBinary != "/usr/local/bin/archbench" {
		t.Fatalf("exec fields not parsed: %#v", s.Targets[0])
	}
}

func writeSpec(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archbench.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
