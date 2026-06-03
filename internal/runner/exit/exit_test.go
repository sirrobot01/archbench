package exit

import (
	"errors"
	"os/exec"
	"testing"
)

// runExit returns the error from a shell that exits with the given code.
func runExit(t *testing.T, code int) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit "+itoa(code)).Run()
	if err == nil {
		t.Fatalf("exit %d produced no error", code)
	}
	return err
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

func TestResult(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		reserved int
		wantCode int
		wantOK   bool
	}{
		{"command failure", runExit(t, 2), SSH, 2, true},
		{"ssh connection failure", runExit(t, SSH), SSH, 0, false},
		{"docker daemon failure", runExit(t, DockerExec), DockerExec, 0, false},
		{"command exit matching other reserved", runExit(t, DockerExec), SSH, DockerExec, true},
		{"non-exit error", errors.New("dial tcp: refused"), SSH, 0, false},
		{"nil error", nil, SSH, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, ok := Result(tt.err, tt.reserved)
			if code != tt.wantCode || ok != tt.wantOK {
				t.Errorf("Result(%v, %d) = (%d, %t), want (%d, %t)",
					tt.err, tt.reserved, code, ok, tt.wantCode, tt.wantOK)
			}
		})
	}
}
