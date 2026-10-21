package generate

import "testing"

func TestPathGrammar(t *testing.T) {
	for _, data := range []string{
		"", "none", "M0 0", "M.5-.5 1e-2+3E2z",
		"M0,0h2v3l4,5c1 2 3 4 5 6s1 2 3 4q1 2 3 4t2 3",
		"M0 0A2 3 45 0110 20a1 2 0 1 0-2-3Z",
	} {
		if err := validatePath(data); err != nil {
			t.Fatalf("valid path %q: %v", data, err)
		}
	}
	for _, data := range []string{
		"broken", "L1 2", "M0", "M0 0 C1 2", "M0 0 X2 3", "M0 0A1 1 0 2 0 2 3",
		"M0 0h1e", "M0 0h1e999", "M0 0Z1", "M0,,0", "M0 0,",
	} {
		if err := validatePath(data); err == nil {
			t.Fatalf("accepted invalid path %q", data)
		}
	}
}
