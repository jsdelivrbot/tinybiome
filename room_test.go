package tinybiome

import (
	"strings"
	"testing"
)

func BenchmarkStringJoin(b *testing.B) {
	j := make([]string, 0)
	for i := 0; i < b.N; i++ {
		j = append(j, "this is quite a long string!")
	}
	b.ResetTimer()
	strings.Join(j, ",")
}
