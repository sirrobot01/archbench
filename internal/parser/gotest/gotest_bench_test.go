package gotest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sirrobot01/archbench/spec"
)

// benchOutput builds realistic `go test -bench -benchmem -count=count` stdout
// for pkgs packages, each with benches benchmarks. This mirrors the shape the
// parser sees on a real run: a pkg: header per package and count samples per
// benchmark line.
func benchOutput(pkgs, benches, count int) string {
	var b strings.Builder
	b.WriteString("goos: linux\ngoarch: amd64\n")
	for p := 0; p < pkgs; p++ {
		fmt.Fprintf(&b, "pkg: github.com/acme/demo/pkg%d\n", p)
		for n := 0; n < benches; n++ {
			for c := 0; c < count; c++ {
				fmt.Fprintf(&b,
					"BenchmarkScenario%d-8\t%d\t%d.%d ns/op\t%d B/op\t%d allocs/op\t%d MB/s\n",
					n, 1000+c, 120+c, c, 48+n, n%4, 16+c)
			}
		}
		b.WriteString("PASS\n")
	}
	return b.String()
}

// testOutput builds realistic `go test -json` stdout for pkgs packages, each
// with tests tests, interleaving run/output/pass events as the go test JSON
// stream does.
func testOutput(pkgs, tests int) string {
	var b strings.Builder
	for p := 0; p < pkgs; p++ {
		pkg := fmt.Sprintf("github.com/acme/demo/pkg%d", p)
		for n := 0; n < tests; n++ {
			name := fmt.Sprintf("TestScenario%d", n)
			fmt.Fprintf(&b, `{"Package":%q,"Action":"run","Test":%q}`+"\n", pkg, name)
			fmt.Fprintf(&b, `{"Package":%q,"Action":"output","Test":%q,"Output":"=== RUN   %s\n"}`+"\n", pkg, name, name)
			fmt.Fprintf(&b, `{"Package":%q,"Action":"pass","Test":%q,"Elapsed":0.0%d}`+"\n", pkg, name, n%9+1)
		}
	}
	return b.String()
}

func BenchmarkParseBench(b *testing.B) {
	p := New()
	out := &spec.Output{Stdout: benchOutput(8, 20, 10)}
	b.ReportAllocs()
	b.SetBytes(int64(len(out.Stdout)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parsed, err := p.Parse(spec.ModeBench, out)
		if err != nil {
			b.Fatal(err)
		}
		if len(parsed.Benchmarks) != 8*20 {
			b.Fatalf("benchmarks = %d, want %d", len(parsed.Benchmarks), 8*20)
		}
	}
}

func BenchmarkParseTest(b *testing.B) {
	p := New()
	out := &spec.Output{Stdout: testOutput(8, 50)}
	b.ReportAllocs()
	b.SetBytes(int64(len(out.Stdout)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parsed, err := p.Parse(spec.ModeTest, out)
		if err != nil {
			b.Fatal(err)
		}
		if len(parsed.Tests) != 8*50 {
			b.Fatalf("tests = %d, want %d", len(parsed.Tests), 8*50)
		}
	}
}
