package spec

// ProtocolVersion is the wire-format version of the exec Job/RunResult exchange.
// An orchestrator stamps it into the Job; a remote `archbench exec` rejects a
// Job whose version it does not understand. With the default auto-bootstrap the
// orchestrator and remote are the same build, so this only guards mismatched
// hand-installed binaries.
const ProtocolVersion = 1

// Job is a self-contained unit of work for a remote `archbench exec` worker:
// everything needed to run one target's suite on the host and return a
// RunResult. It carries no target type or transport detail, so the remote side
// always runs the suite locally regardless of how the orchestrator reached it.
type Job struct {
	ProtocolVersion int               `json:"protocol_version"`
	Mode            Mode              `json:"mode"`
	Parser          string            `json:"parser"`
	Setup           []string          `json:"setup,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	Runs            []Run             `json:"runs"`
	Cache           Cache             `json:"cache"`
}
