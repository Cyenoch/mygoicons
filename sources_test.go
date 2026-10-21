package mygoicons_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cyenoch/mygoicons/generate"
)

func TestBundledSources(t *testing.T) {
	var lock struct {
		SchemaVersion int
		Collections   []struct {
			Prefix string
			Files  map[string]struct {
				SHA256 string
				Bytes  int
			}
			Counts  struct{ Icons, Aliases int }
			License struct {
				Notices []struct {
					File, SHA256 string
					Bytes        int
				}
			}
		}
	}
	read := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	if err := json.Unmarshal(read("sources/lock.json"), &lock); err != nil {
		t.Fatal(err)
	}
	if lock.SchemaVersion != 1 || len(lock.Collections) != 2 {
		t.Fatal("unexpected source lock schema or collection count")
	}
	for _, set := range lock.Collections {
		t.Run(set.Prefix, func(t *testing.T) {
			check := func(name, sum string, size int) []byte {
				t.Helper()
				data := read(filepath.Join("sources", set.Prefix, name))
				if len(data) != size || fmt.Sprintf("%x", sha256.Sum256(data)) != sum {
					t.Fatalf("%s source differs from verified lock", name)
				}
				return data
			}
			for name, file := range set.Files {
				check(name, file.SHA256, file.Bytes)
			}
			for _, notice := range set.License.Notices {
				check(notice.File, notice.SHA256, notice.Bytes)
			}
			result, err := generate.Build(read(filepath.Join("sources", set.Prefix, "icons.json")), generate.Options{Package: set.Prefix})
			if err != nil {
				t.Fatal(err)
			}
			if result.Count != set.Counts.Icons+set.Counts.Aliases {
				t.Fatalf("generated %d names; source lock expects %d", result.Count, set.Counts.Icons+set.Counts.Aliases)
			}
			if !bytes.Equal(result.Source, read(filepath.Join(set.Prefix, "icons_gen.go"))) {
				t.Fatal("checked-in source is stale; run go generate ./")
			}
		})
	}
}
