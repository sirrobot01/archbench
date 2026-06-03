package cli

import (
	"strings"
	"testing"

	"github.com/sirrobot01/archbench"
)

func TestGenerateWorkflow(t *testing.T) {
	s := &archbench.Spec{
		Name:   "svc-bench",
		Parser: "go-test",
		Targets: []archbench.Target{
			{Name: "local", Type: archbench.TargetLocal},
			{Name: "ci-amd64", Type: archbench.TargetGitHubActions, RunsOn: "ubuntu-latest"},
			{Name: "ci-arm64", Type: archbench.TargetGitHubActions, RunsOn: "ubuntu-24.04-arm"},
		},
	}

	got, err := generateWorkflow(s, "archbench.yaml")
	if err != nil {
		t.Fatalf("generateWorkflow: %v", err)
	}

	for _, want := range []string{
		"name: svc-bench",
		"- target: ci-amd64",
		"runs-on: ubuntu-latest",
		"- target: ci-arm64",
		"runs-on: ubuntu-24.04-arm",
		"uses: actions/setup-go@v5", // go-test parser pulls in Go setup
		"archbench run --spec archbench.yaml --target ${{ matrix.target }}",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("workflow missing %q\n---\n%s", want, got)
		}
	}
	// Non-CI targets are not turned into jobs.
	if strings.Contains(got, "target: local") {
		t.Errorf("local target should not appear in the workflow matrix:\n%s", got)
	}
}

func TestGenerateWorkflowDefaultsRunsOn(t *testing.T) {
	s := &archbench.Spec{
		Name:    "svc",
		Parser:  "go-test",
		Targets: []archbench.Target{{Name: "ci", Type: archbench.TargetGitHubActions}},
	}
	got, err := generateWorkflow(s, "archbench.yaml")
	if err != nil {
		t.Fatalf("generateWorkflow: %v", err)
	}
	if !strings.Contains(got, "runs-on: ubuntu-latest") {
		t.Errorf("expected default runner ubuntu-latest:\n%s", got)
	}
}

func TestGenerateWorkflowNoTargets(t *testing.T) {
	s := &archbench.Spec{
		Name:    "svc",
		Parser:  "go-test",
		Targets: []archbench.Target{{Name: "local", Type: archbench.TargetLocal}},
	}
	if _, err := generateWorkflow(s, "archbench.yaml"); err == nil {
		t.Fatal("expected an error when no github-actions targets are present")
	}
}
