package join

import (
	"strings"
	"testing"
)

func TestConcatMatchesBuilder(t *testing.T) {
	parts := []string{"a", "b", "c", "d"}
	if got := Concat(parts); got != "abcd" {
		t.Fatalf("Concat = %q, want abcd", got)
	}
	if Concat(parts) != Builder(parts) {
		t.Fatal("Concat and Builder disagree")
	}
}

// BenchmarkConcat and BenchmarkBuilder produce very different B/op and
// allocs/op, which is the point of the example: -benchmem makes those columns
// worth comparing across targets.
func BenchmarkConcat(b *testing.B)  { benchmarkJoin(b, Concat) }
func BenchmarkBuilder(b *testing.B) { benchmarkJoin(b, Builder) }

func benchmarkJoin(b *testing.B, join func([]string) string) {
	parts := make([]string, 256)
	for i := range parts {
		parts[i] = strings.Repeat("x", 8)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = join(parts)
	}
}
