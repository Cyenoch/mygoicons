package generate

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// MyGo accepts malformed path data without returning an error. Validate command
// arities and finite numbers before allowing an SVG into generated source.
// Grammar reference: https://www.w3.org/TR/SVG/paths.html#PathDataBNF
func validatePath(data string) error {
	s := pathScanner{data: strings.TrimSpace(data)}
	if s.data == "" || s.data == "none" {
		return nil
	}
	first := true
	for s.pos < len(s.data) {
		command := s.data[s.pos]
		s.pos++
		lower := command | 0x20
		if first && lower != 'm' {
			return fmt.Errorf("SVG path must start with moveto")
		}
		first = false
		arity := 0
		switch lower {
		case 'm', 'l', 't':
			arity = 2
		case 'h', 'v':
			arity = 1
		case 's', 'q':
			arity = 4
		case 'c':
			arity = 6
		case 'a':
			arity = 7
		case 'z':
			s.space()
			continue
		default:
			return fmt.Errorf("unknown SVG path command %q", command)
		}
		groups := 0
		for {
			for argument := range arity {
				s.space()
				if s.pos < len(s.data) && s.data[s.pos] == ',' && (argument > 0 || groups > 0) {
					s.pos++
					s.space()
				}
				if lower == 'a' && (argument == 3 || argument == 4) {
					if s.pos >= len(s.data) || (s.data[s.pos] != '0' && s.data[s.pos] != '1') {
						return fmt.Errorf("SVG arc flag must be 0 or 1")
					}
					s.pos++
				} else if err := s.number(); err != nil {
					return err
				}
			}
			groups++
			s.space()
			if s.pos == len(s.data) || strings.ContainsRune("MmLlHhVvCcSsQqTtAaZz", rune(s.data[s.pos])) {
				break
			}
		}
	}
	return nil
}

type pathScanner struct {
	data string
	pos  int
}

func (s *pathScanner) space() {
	for s.pos < len(s.data) && strings.ContainsRune(" \t\r\n", rune(s.data[s.pos])) {
		s.pos++
	}
}

func (s *pathScanner) digits() int {
	start := s.pos
	for s.pos < len(s.data) && s.data[s.pos] >= '0' && s.data[s.pos] <= '9' {
		s.pos++
	}
	return s.pos - start
}

func (s *pathScanner) number() error {
	start := s.pos
	if s.pos < len(s.data) && (s.data[s.pos] == '+' || s.data[s.pos] == '-') {
		s.pos++
	}
	digits := s.digits()
	if s.pos < len(s.data) && s.data[s.pos] == '.' {
		s.pos++
		digits += s.digits()
	}
	if digits == 0 {
		return fmt.Errorf("missing SVG path number at byte %d", start)
	}
	if s.pos < len(s.data) && (s.data[s.pos] == 'e' || s.data[s.pos] == 'E') {
		s.pos++
		if s.pos < len(s.data) && (s.data[s.pos] == '+' || s.data[s.pos] == '-') {
			s.pos++
		}
		if s.digits() == 0 {
			return fmt.Errorf("invalid SVG path exponent at byte %d", start)
		}
	}
	n, err := strconv.ParseFloat(s.data[start:s.pos], 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > 1e9 {
		return fmt.Errorf("SVG path number at byte %d must be finite and within +/-1e9", start)
	}
	return nil
}
