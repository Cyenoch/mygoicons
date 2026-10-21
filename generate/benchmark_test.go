package generate

import (
	"os"
	"testing"
)

func BenchmarkBuildCollections(b *testing.B) {
	for _, prefix := range []string{"lucide", "twemoji"} {
		b.Run(prefix, func(b *testing.B) {
			data, err := os.ReadFile("../sources/" + prefix + "/icons.json")
			if err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := Build(data, Options{Package: prefix}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
