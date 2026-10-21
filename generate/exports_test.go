package generate

import (
	"bytes"
	"testing"
)

func TestExportCompatibility(t *testing.T) {
	old, err := Build([]byte(`{"prefix":"test","icons":{"old-name":{"body":"<path d=\"M0 0h1\"/>"}}}`), Options{Package: "icons"})
	if err != nil {
		t.Fatal(err)
	}
	previous, err := ReadExports(old.Source)
	if err != nil {
		t.Fatal(err)
	}
	renamed := []byte(`{"prefix":"test","icons":{"new-name":{"body":"<path d=\"M0 0h1\"/>"}}}`)
	next, err := Build(renamed, Options{Package: "icons", Previous: previous})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(next.Source, []byte("func OldName() *ui.SVG { return NewName() }")) {
		t.Fatal("old name not preserved as a direct alias")
	}
	parsed, err := ReadExports(next.Source)
	if err != nil || len(parsed) != 2 {
		t.Fatal(parsed, err)
	}
	again, err := Build(renamed, Options{Package: "icons", Previous: parsed})
	if err != nil || !bytes.Equal(next.Source, again.Source) {
		t.Fatal("compatibility aliases not reproducible", err)
	}
	// An initialism policy change can change a symbol while its ID remains stable.
	previous[0].Name = "OLDName"
	renamedPolicy, err := Build([]byte(`{"prefix":"test","icons":{"old-name":{"body":"<path d=\"M0 0h2\"/>"}}}`), Options{Package: "icons", Previous: previous})
	if err != nil || !bytes.Contains(renamedPolicy.Source, []byte("func OLDName()")) {
		t.Fatal("policy rename lost", err)
	}
	for _, data := range []string{
		`{"prefix":"test","icons":{"different":{"body":"<path d=\"M0 0h2\"/>"}}}`,
		`{"prefix":"test","icons":{"a":{"body":"<path d=\"M0 0h1\"/>"},"b":{"body":"<path d=\"M0 0h1\"/>"}}}`,
		`{"prefix":"test","icons":{"api":{"body":"<path/>"},"a-p-i":{"body":"<circle/>"}}}`,
	} {
		if _, err := Build([]byte(data), Options{Package: "icons", Previous: old.Exports}); err == nil {
			t.Fatalf("accepted incompatible change %s", data)
		}
	}
	// Reusing an old symbol for different artwork cannot silently redirect callers.
	p := []Export{{Name: "NewName", ID: "old-name", SHA256: old.Exports[0].SHA256}}
	if _, err := Build([]byte(`{"prefix":"test","icons":{"old-name":{"body":"<path d=\"M0 0h1\"/>"},"new-name":{"body":"<circle/>"}}}`), Options{Package: "icons", Previous: p}); err == nil {
		t.Fatal("accepted identity conflict")
	}
}
