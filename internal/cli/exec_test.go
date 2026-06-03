package cli

import (
	"bytes"
	"encoding/json"
	"runtime"
	"testing"

	"github.com/sirrobot01/archbench"
)

// runExec drives the hidden exec worker with a job on stdin and returns its
// decoded result, mirroring what the SSH runner does over the wire.
func runExec(t *testing.T, job archbench.Job) (*archbench.RunResult, error) {
	t.Helper()
	in, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}

	cmd := newExecCmd()
	cmd.SetIn(bytes.NewReader(in))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dir", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		return nil, err
	}

	var res archbench.RunResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("decode result: %v\n%s", err, out.String())
	}
	return &res, nil
}

// TestExecRoundTrip confirms the worker decodes a job, runs it locally, and
// emits a RunResult on stdout -- the foundation of the remote exec path.
func TestExecRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("local runner requires a POSIX shell")
	}

	res, err := runExec(t, archbench.Job{
		ProtocolVersion: archbench.ProtocolVersion,
		Mode:            archbench.ModeBench,
		Parser:          "go-test",
		Runs: []archbench.Run{
			{Name: "first", Command: "echo one"},
			{Name: "second", Command: "echo two"},
		},
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if len(res.Runs) != 2 || res.Runs[0].Name != "first" || res.Runs[1].Name != "second" {
		t.Fatalf("unexpected runs: %#v", res.Runs)
	}
	if res.Runs[0].ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", res.Runs[0].ExitCode)
	}
}

// TestExecCapturesFailure confirms a failed run carries its exit code and
// stderr back through the worker, so the orchestrator can surface it.
func TestExecCapturesFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("local runner requires a POSIX shell")
	}

	res, err := runExec(t, archbench.Job{
		ProtocolVersion: archbench.ProtocolVersion,
		Mode:            archbench.ModeBench,
		Parser:          "go-test",
		Runs:            []archbench.Run{{Name: "boom", Command: "echo nope 1>&2; exit 3"}},
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if res.Runs[0].ExitCode != 3 {
		t.Errorf("exit code = %d, want 3", res.Runs[0].ExitCode)
	}
	if res.Runs[0].Stderr == "" {
		t.Error("expected stderr to be captured for a failed run")
	}
}

// TestExecRejectsProtocolMismatch confirms the worker refuses a job whose
// protocol version it does not understand.
func TestExecRejectsProtocolMismatch(t *testing.T) {
	_, err := runExec(t, archbench.Job{
		ProtocolVersion: archbench.ProtocolVersion + 1,
		Mode:            archbench.ModeBench,
		Parser:          "go-test",
		Runs:            []archbench.Run{{Name: "noop", Command: "true"}},
	})
	if err == nil {
		t.Fatal("expected a protocol mismatch error")
	}
}
