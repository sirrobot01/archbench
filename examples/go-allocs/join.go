// Package join builds one string from many parts using two strategies whose
// allocation profiles differ sharply. It makes the B/op and allocs/op columns
// meaningful, so reports and cross-architecture comparisons have something to
// show beyond ns/op.
package join

import "strings"

// Concat joins parts with naive += concatenation, reallocating the result on
// every step.
func Concat(parts []string) string {
	out := ""
	for _, p := range parts {
		out += p
	}
	return out
}

// Builder joins parts with a strings.Builder, which amortizes growth and so
// allocates far less than Concat.
func Builder(parts []string) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p)
	}
	return b.String()
}
