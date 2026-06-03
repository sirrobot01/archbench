package archbench

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

// DefaultSpecFile is the spec filename resolved when none is given.
const DefaultSpecFile = "archbench.yaml"

// Mode selects how a run's output is interpreted and compared.
type Mode string

const (
	ModeBench Mode = "bench"
	ModeTest  Mode = "test"
)

// TargetType selects the runner used for a target.
type TargetType string

const (
	TargetLocal         TargetType = "local"
	TargetSSH           TargetType = "ssh"
	TargetDocker        TargetType = "docker"
	TargetGitHubActions TargetType = "github-actions"
)

// Spec is a benchmark or test suite definition.
type Spec struct {
	Name    string   `yaml:"name"`
	Mode    Mode     `yaml:"mode,omitempty"`
	Targets []Target `yaml:"targets"`
	Runs    []Run    `yaml:"runs,omitempty"`
	Parser  string   `yaml:"parser,omitempty"`
}

// Target is a single execution environment.
type Target struct {
	Name string     `yaml:"name"`
	Type TargetType `yaml:"type"`

	// Setup holds provisioning steps run once per target, after the project is
	// uploaded but before any run group. Use it for environment provisioning
	// that should not repeat for every run -- installing system packages or
	// warming the module cache. Per-run preparation belongs in Run.Setup.
	Setup []string `yaml:"setup,omitempty"`

	// Env holds environment variables applied to the target's setup steps and
	// to every run on it, as a base a run's own Env overrides. Values may
	// reference the cache directory through $ARCHBENCH_CACHE. Like Run.Env, the
	// values may be secrets and are never exposed on a command line.
	Env map[string]string `yaml:"env,omitempty"`

	// Exec runs the suite through a remote `archbench exec` worker instead of
	// driving raw shell commands: the project is uploaded, the runs execute on
	// the host, and a structured result is returned. The remote side detects
	// its own toolchain and parses output where the commands run. SSH only.
	Exec bool `yaml:"exec,omitempty"`

	// ExecBinary is the path to the archbench binary on the host, used with
	// Exec. When empty, archbench is located on the host (or installed with
	// `go install` when the orchestrator is a released version).
	ExecBinary string `yaml:"execBinary,omitempty"`

	// Host may be a hostname or a ~/.ssh/config alias. User, Port, Key and
	// ProxyJump are optional overrides; anything left unset is resolved by the
	// system ssh client and the user's ssh config.
	Host      string `yaml:"host,omitempty"`
	User      string `yaml:"user,omitempty"`
	Port      int    `yaml:"port,omitempty"`
	Key       string `yaml:"key,omitempty"`
	ProxyJump string `yaml:"proxyJump,omitempty"`

	// Image and Platform configure a docker target. Platform (e.g. linux/amd64)
	// pins a non-native architecture through the daemon's emulation.
	Image    string `yaml:"image,omitempty"`
	Platform string `yaml:"platform,omitempty"`

	// RunsOn names the GitHub-hosted runner label for a github-actions target,
	// e.g. ubuntu-latest or ubuntu-24.04-arm. It feeds `archbench generate`'s
	// workflow matrix and defaults to ubuntu-latest.
	RunsOn string `yaml:"runsOn,omitempty"`
}

// Run holds one named command group executed on each target.
type Run struct {
	Name    string            `yaml:"name,omitempty" json:"name,omitempty"`
	Setup   []string          `yaml:"setup,omitempty" json:"setup,omitempty"`
	Command string            `yaml:"command" json:"command"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// EffectiveMode returns the mode, defaulting to ModeBench.
func (s Spec) EffectiveMode() Mode {
	if s.Mode == "" {
		return ModeBench
	}
	return s.Mode
}

// LoadSpec reads, defaults, and validates the spec at path.
func LoadSpec(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}

	var s Spec
	if err := yaml.UnmarshalWithOptions(data, &s, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}

	s.applyDefaults()
	if err := s.validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *Spec) applyDefaults() {
	if s.Mode == "" {
		s.Mode = ModeBench
	}
	if s.Parser == "" {
		s.Parser = "go-test"
	}
}

func (s *Spec) validate() error {
	if s.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch s.Mode {
	case ModeBench, ModeTest:
	default:
		return fmt.Errorf("unknown mode %q", s.Mode)
	}
	if len(s.Targets) == 0 {
		return fmt.Errorf("at least one target is required")
	}
	if len(s.Runs) == 0 {
		return fmt.Errorf("at least one run is required")
	}
	seenRuns := make(map[string]bool, len(s.Runs))
	for i, run := range s.Runs {
		if run.Command == "" {
			return fmt.Errorf("runs[%d]: command is required", i)
		}
		if run.Name == "" {
			return fmt.Errorf("runs[%d]: name is required", i)
		}
		if seenRuns[run.Name] {
			return fmt.Errorf("duplicate run name %q", run.Name)
		}
		seenRuns[run.Name] = true
	}

	seen := make(map[string]bool, len(s.Targets))
	for i, t := range s.Targets {
		if t.Name == "" {
			return fmt.Errorf("targets[%d]: name is required", i)
		}
		if seen[t.Name] {
			return fmt.Errorf("duplicate target name %q", t.Name)
		}
		seen[t.Name] = true

		switch t.Type {
		case TargetLocal, TargetGitHubActions:
		case TargetSSH:
			if t.Host == "" {
				return fmt.Errorf("target %q: host is required", t.Name)
			}
		case TargetDocker:
			if t.Image == "" {
				return fmt.Errorf("target %q: image is required", t.Name)
			}
		default:
			return fmt.Errorf("target %q: unknown type %q", t.Name, t.Type)
		}

		if t.Exec && t.Type != TargetSSH {
			return fmt.Errorf("target %q: exec is only supported for ssh targets", t.Name)
		}
		if t.ExecBinary != "" && !t.Exec {
			return fmt.Errorf("target %q: execBinary requires exec: true", t.Name)
		}
	}
	return nil
}
