package archbench

import "fmt"

// Parser normalizes the raw output of a run into benchmarks or tests. It is the
// only language-specific component; a parser declares which modes it supports.
type Parser interface {
	Name() string
	Modes() []Mode
	Parse(mode Mode, out *Output) (*Parsed, error)
}

// Parsed holds a parser's output. One slice is populated, keyed by mode.
type Parsed struct {
	Benchmarks []Benchmark
	Tests      []Test
	Toolchain  map[string]string
}

// Supports reports whether p can parse the given mode.
func Supports(p Parser, mode Mode) bool {
	for _, m := range p.Modes() {
		if m == mode {
			return true
		}
	}
	return false
}

// Registry maps parser names to implementations.
type Registry struct {
	parsers map[string]Parser
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{parsers: make(map[string]Parser)}
}

// Register adds p, keyed by its name. It panics on a duplicate name.
func (r *Registry) Register(p Parser) {
	name := p.Name()
	if _, dup := r.parsers[name]; dup {
		panic(fmt.Sprintf("parser already registered: %q", name))
	}
	r.parsers[name] = p
}

// Get returns the parser registered under name.
func (r *Registry) Get(name string) (Parser, bool) {
	p, ok := r.parsers[name]
	return p, ok
}
