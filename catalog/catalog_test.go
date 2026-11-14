package catalog_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Cyenoch/mygoicons/catalog"
)

func TestCollectionsListUsableIcons(t *testing.T) {
	for _, set := range catalog.All() {
		if set.Prefix == "" || set.Title == "" || set.Version == "" || set.License == "" || len(set.Icons) == 0 {
			t.Fatalf("incomplete collection: %+v", set)
		}
		byName := make(map[string]catalog.Icon, len(set.Icons))
		for _, icon := range set.Icons {
			if _, repeated := byName[icon.Name]; repeated || icon.ID == "" || icon.SVG == nil {
				t.Fatalf("%s: bad entry %q (%q)", set.Prefix, icon.Name, icon.ID)
			}
			byName[icon.Name] = icon
		}
		for _, icon := range set.Icons {
			if icon.AliasOf == "" {
				continue
			}
			// A browser that skips aliases must still show the drawing.
			if original, ok := byName[icon.AliasOf]; !ok || original.AliasOf != "" {
				t.Fatalf("%s: %s is an alias of %q, which is not a drawing of its own", set.Prefix, icon.Name, icon.AliasOf)
			}
		}
	}
	if !catalog.Twemoji().Colored || catalog.Lucide().Colored {
		t.Fatal("Twemoji keeps its colors and Lucide follows the text color")
	}
	if width, height := catalog.Lucide().Icons[0].SVG().Size(); width <= 0 || height <= 0 {
		t.Fatalf("first Lucide icon has no size: %v x %v", width, height)
	}
}

// Listing one collection must not carry the other into the executable:
// Twemoji is ten times Lucide's size.
func TestListingLucideLeavesTwemojiOut(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "probe.exe")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "./testdata/lucideonly")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`viewBox=\"0 0 24 24\"`)) && !bytes.Contains(data, []byte(`viewBox="0 0 24 24"`)) {
		t.Fatal("the probe holds no Lucide artwork; the check below would prove nothing")
	}
	if bytes.Contains(data, []byte(`viewBox="0 0 36 36"`)) {
		t.Fatal("listing Lucide linked Twemoji artwork")
	}
}
