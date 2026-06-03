// Package exit classifies process exit codes from the intermediary CLIs the
// SSH and Docker runners drive (`ssh`, `docker exec`). Each reserves one exit
// code for its own transport or daemon failures; every other non-zero code
// belongs to the user's command and is a test or benchmark result, not a runner
// error. Centralizing the rule keeps the two runners consistent and contains
// the one brittle spot in shelling out rather than using a client library.
package exit

import (
	"errors"
	"os/exec"
)

// SSH is the exit code ssh returns for its own (connection) failures.
const SSH = 255

// DockerExec is the exit code `docker exec` returns for its own (daemon)
// failures.
const DockerExec = 125

// Result reports the command's exit code and whether the failure is the
// command's own. reserved is the code the intermediary uses for its own
// failures. When ok is false, err is not a command result -- either it is not a
// process exit at all or it is the intermediary's reserved failure code -- and
// the caller should surface it as a runner error.
func Result(err error, reserved int) (code int, ok bool) {
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		if c := ee.ExitCode(); c > 0 && c != reserved {
			return c, true
		}
	}
	return 0, false
}
