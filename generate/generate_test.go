package generate

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestOfficialOracle(t *testing.T) {
	var provenance struct {
		Files map[string]struct {
			SHA256 string
			Bytes  int
		}
	}
	loadJSON(t, "../testdata/upstream/provenance.json", &provenance)
	for name, file := range provenance.Files {
		data, err := os.ReadFile("../testdata/upstream/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != file.Bytes || fmt.Sprintf("%x", sha256.Sum256(data)) != file.SHA256 {
			t.Fatalf("upstream oracle %s differs from provenance", name)
		}
	}
	var input struct {
		Collection json.RawMessage
		Cases      []struct {
			ID             string
			Icon           string
			Customisations json.RawMessage
		}
	}
	var expected struct {
		Results []struct {
			ID       string
			Resolved *properties
			SVG      *struct {
				Attributes struct{ ViewBox string }
				Body       string
			}
		}
	}
	loadJSON(t, "../testdata/upstream/alias-transform.input.json", &input)
	loadJSON(t, "../testdata/upstream/alias-transform.expected.json", &expected)
	c, err := readCollection(input.Collection)
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Cases) != len(expected.Results) {
		t.Fatal("oracle cases and results differ")
	}
	for i, test := range input.Cases {
		t.Run(test.ID, func(t *testing.T) {
			want := expected.Results[i]
			if want.ID != test.ID {
				t.Fatal("oracle order differs")
			}
			r := resolver{c, make(map[string]properties), make(map[string]bool)}
			got, err := r.resolve(test.Icon, 0)
			if want.Resolved == nil {
				if err == nil {
					t.Fatal("invalid upstream icon accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			icon, err := c.complete(got)
			if err != nil {
				t.Fatal(err)
			}
			// The official resolver already applied collection defaults.
			wantIcon, err := (&collection{}).complete(*want.Resolved)
			if err != nil || !reflect.DeepEqual(icon, wantIcon) {
				t.Fatalf("resolve: got %+v, want %+v, err %v", icon, wantIcon, err)
			}
			if len(test.Customisations) != 0 {
				// Caller web customisations are outside the Go generator API.
				// Resolution above still checks this oracle case.
				return
			}
			doc, box, err := icon.svg()
			if err != nil {
				t.Fatal(err)
			}
			body := doc[strings.IndexByte(doc, '>')+1 : strings.LastIndex(doc, "</svg>")]
			if box != want.SVG.Attributes.ViewBox || body != want.SVG.Body {
				t.Fatalf("SVG differs from official Utils:\n%s\nwant box %s, body %s", doc, want.SVG.Attributes.ViewBox, want.SVG.Body)
			}
		})
	}
}

func loadJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}

func testCollection(body string) []byte {
	data, _ := json.Marshal(map[string]any{
		"prefix": "test",
		"icons":  map[string]any{"sample": map[string]any{"body": body}},
	})
	return data
}

func TestBuildDeterministicAndSubset(t *testing.T) {
	data := []byte(`{"prefix":"test","icons":{"b":{"body":"<circle r=\"1\"/>"},"a":{"body":"<path d=\"M0 0h1\"/>"}}}`)
	all, err := Build(data, Options{Package: "fa6solid"})
	if err != nil {
		t.Fatal(err)
	}
	again, err := Build(data, Options{Package: "fa6solid"})
	if err != nil || !bytes.Equal(all.Source, again.Source) || all.Count != 2 {
		t.Fatal("generation is not deterministic", err)
	}
	selected, err := Build(data, Options{Package: "icons", Icons: []string{"b", "b"}})
	if err != nil || selected.Count != 1 || bytes.Contains(selected.Source, []byte("func A()")) {
		t.Fatal("subset selected unwanted icons", err)
	}
	if _, err := Build(data, Options{Package: "icons", Icons: []string{"typo"}}); err == nil || !strings.Contains(err.Error(), "test:typo") {
		t.Fatal("missing icon must identify original ID", err)
	}
}

func TestInvalidInput(t *testing.T) {
	tests := []struct {
		name, input, problem string
	}{
		{"json", `{`, "decode"},
		{"empty", `{}`, "prefix"},
		{"missing-body", `{"prefix":"x","icons":{"a":{}}}`, "body"},
		{"collision", `{"prefix":"x","icons":{"api":{"body":"<path/>"},"a-p-i":{"body":"<path/>"}}}`, "both generate"},
		{"zero-width", `{"prefix":"x","width":0,"icons":{"a":{"body":"<path/>"}}}`, "positive"},
		{"non-finite", `{"prefix":"x","width":1e999,"icons":{"a":{"body":"<path/>"}}}`, "decode"},
		{"too-large-coordinate", `{"prefix":"x","width":1048577,"icons":{"a":{"body":"<path/>"}}}`, "coordinates"},
		{"duplicate-name", `{"prefix":"x","icons":{"a":{"body":"<path/>"}},"aliases":{"a":{"parent":"a"}}}`, "both an icon"},
		{"cycle", `{"prefix":"x","icons":{"a":{"body":"<path/>"}},"aliases":{"b":{"parent":"c"},"c":{"parent":"b"}}}`, "cycle"},
		{"parent", `{"prefix":"x","icons":{"a":{"body":"<path/>"}},"aliases":{"b":{"parent":"missing"}}}`, "unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Build([]byte(test.input), Options{Package: "icons"})
			if err == nil || !strings.Contains(err.Error(), test.problem) {
				t.Fatalf("want %q error, got %v", test.problem, err)
			}
		})
	}
	for _, pkg := range []string{"", "main", "_", "type", "my-icons", "MyIcons", "1icons"} {
		if _, err := Build(testCollection("<path/>"), Options{Package: pkg}); err == nil {
			t.Fatalf("accepted package %q", pkg)
		}
	}
}

func TestSVGCompatibilityErrors(t *testing.T) {
	tests := []string{
		`<text>not paths</text>`, `<style>.a { fill: red }</style>`,
		`<filter/>`, `<image href="https://example.com/a.png"/>`,
		`<use href="other.svg#a"/>`, `<path fill="url(https://example.com/x)"/>`,
		`<path style="filter:blur(1px)"/>`, `<path vector-effect="non-scaling-stroke"/>`,
		`<path onload="run()"/>`, `<path d="broken"/>`, `</svg><svg>`,
		`<!DOCTYPE svg><path/>`, `<path>text</path>`,
		`<g>` + strings.Repeat(`<g>`, 128) + strings.Repeat(`</g>`, 129),
	}
	for _, body := range tests {
		t.Run(body[:min(45, len(body))], func(t *testing.T) {
			_, err := Build(testCollection(body), Options{Package: "icons"})
			if err == nil || !strings.Contains(err.Error(), "test:sample") {
				t.Fatalf("unsupported SVG must report original ID: %v", err)
			}
		})
	}
}

func TestEmptyDefinitionsAndSizeBounds(t *testing.T) {
	if _, err := Build(testCollection("<path/>"), Options{Package: "icons", Icons: make([]string, MaxIcons+1)}); err == nil {
		t.Fatal("accepted an oversized selection before deduplication")
	}
	for _, body := range []string{`<defs/><path d="M0 0h1"/>`, `<defs></defs><path d="M0 0h1"/>`} {
		data := strings.Replace(string(testCollection(body)), `"body":`, `"hFlip":true,"body":`, 1)
		if _, err := Build([]byte(data), Options{Package: "icons"}); err != nil {
			t.Fatal(err)
		}
	}
	// Real logo bodies exceed 64 KiB; the bounded generator must accept them.
	body := `<path d="M0 0h1"/>` + strings.Repeat(" ", 160_000)
	if _, err := Build(testCollection(body), Options{Package: "icons"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(testCollection(strings.Repeat(" ", MaxBodyBytes+1)), Options{Package: "icons"}); err == nil {
		t.Fatal("accepted oversized body")
	}
	aliases := make(map[string]properties)
	for i := range 140 {
		aliases[fmt.Sprintf("alias-%03d", i)] = properties{Parent: fmt.Sprintf("alias-%03d", i+1)}
	}
	bodyText := `<path/>`
	data, _ := json.Marshal(collection{Prefix: "test", Icons: map[string]properties{"alias-140": {Body: &bodyText}}, Aliases: aliases})
	if _, err := Build(data, Options{Package: "icons"}); err == nil || !strings.Contains(err.Error(), "levels") {
		t.Fatal("accepted unbounded alias chain", err)
	}
	// Parents that sort before their children are already cached. The chain
	// limit must hold independently of that traversal order.
	aliases = make(map[string]properties)
	for i := 1; i <= 140; i++ {
		aliases[fmt.Sprintf("alias-%03d", i)] = properties{Parent: fmt.Sprintf("alias-%03d", i-1)}
	}
	data, _ = json.Marshal(collection{Prefix: "test", Icons: map[string]properties{"alias-000": {Body: &bodyText}}, Aliases: aliases})
	if _, err := Build(data, Options{Package: "icons"}); err == nil || !strings.Contains(err.Error(), "levels") {
		t.Fatal("cache order bypassed the alias chain bound", err)
	}
}

func TestNumericNames(t *testing.T) {
	for name, want := range map[string]string{
		"arrow-down-0-1": "ArrowDown0_1", "arrow-down-01": "ArrowDown01",
		"1st-place-medal": "Icon1stPlaceMedal", "scan-qr-code": "ScanQRCode",
	} {
		if got := exportedName(name); got != want {
			t.Fatalf("%s: got %s want %s", name, got, want)
		}
	}
}

func FuzzBuild(f *testing.F) {
	f.Add(testCollection(`<defs/><path d="M0 0h1"/>`))
	f.Add([]byte(`{"prefix":"x","icons":{"a":{"body":"<path/>","rotate":1}}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 32<<10 {
			return
		}
		result, err := Build(data, Options{Package: "icons"})
		if err == nil && len(result.Source) == 0 {
			t.Fatal("successful generation produced no source")
		}
	})
}
