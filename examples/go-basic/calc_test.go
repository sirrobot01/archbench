package calc

import "testing"

func TestSum(t *testing.T) {
	got := Sum([]int{1, 2, 3, 4})
	if got != 10 {
		t.Fatalf("Sum() = %d, want 10", got)
	}
}

func BenchmarkSumSmall(b *testing.B) {
	benchmarkSum(b, 128)
}

func BenchmarkSumLarge(b *testing.B) {
	benchmarkSum(b, 1024)
}

func benchmarkSum(b *testing.B, size int) {
	values := make([]int, size)
	for i := range values {
		values[i] = i
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Sum(values)
	}
}
