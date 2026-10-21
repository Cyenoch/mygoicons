package generate

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
)

// Limits bound the generator's input and output, rather than its heap usage.
const (
	MaxInputBytes  = 64 << 20
	MaxIcons       = 50_000
	MaxBodyBytes   = 1 << 20
	MaxOutputBytes = 128 << 20
	maxAliasDepth  = 128
)

var iconName = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type dimensions struct {
	Left   *float64 `json:"left"`
	Top    *float64 `json:"top"`
	Width  *float64 `json:"width"`
	Height *float64 `json:"height"`
}

type properties struct {
	dimensions
	Body       *string `json:"body"`
	Parent     string  `json:"parent"`
	Rotate     *int    `json:"rotate"`
	HFlip      *bool   `json:"hFlip"`
	VFlip      *bool   `json:"vFlip"`
	aliasDepth int
}

type collection struct {
	dimensions
	Prefix  string                `json:"prefix"`
	Icons   map[string]properties `json:"icons"`
	Aliases map[string]properties `json:"aliases"`
}

type resolvedIcon struct {
	body                     string
	left, top, width, height float64
	rotate                   int
	hFlip, vFlip             bool
}

func readCollection(data []byte) (*collection, error) {
	if len(data) > MaxInputBytes {
		return nil, fmt.Errorf("collection exceeds %d bytes", MaxInputBytes)
	}
	var c collection
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("decode Iconify collection: %w", err)
	}
	if !iconName.MatchString(c.Prefix) || len(c.Prefix) > 128 {
		return nil, fmt.Errorf("invalid collection prefix %q", c.Prefix)
	}
	count := len(c.Icons) + len(c.Aliases)
	if len(c.Icons) == 0 || count > MaxIcons {
		return nil, fmt.Errorf("%s: expected icons and at most %d total names", c.Prefix, MaxIcons)
	}
	for _, name := range c.names() {
		if !iconName.MatchString(name) || len(name) > 256 {
			return nil, fmt.Errorf("%s: invalid icon name %q", c.Prefix, name)
		}
		if _, icon := c.Icons[name]; icon {
			if _, alias := c.Aliases[name]; alias {
				return nil, fmt.Errorf("%s:%s: name is both an icon and alias", c.Prefix, name)
			}
		}
	}
	return &c, nil
}

func (c *collection) names() []string {
	names := make([]string, 0, len(c.Icons)+len(c.Aliases))
	for name := range c.Icons {
		names = append(names, name)
	}
	for name := range c.Aliases {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

type resolver struct {
	collection *collection
	cache      map[string]properties
	active     map[string]bool
}

func (r *resolver) resolve(name string, depth int) (properties, error) {
	if depth > maxAliasDepth {
		return properties{}, fmt.Errorf("alias chain exceeds %d levels", maxAliasDepth)
	}
	if p, ok := r.cache[name]; ok {
		if depth+p.aliasDepth > maxAliasDepth {
			return properties{}, fmt.Errorf("alias chain exceeds %d levels", maxAliasDepth)
		}
		return p, nil
	}
	if r.active[name] {
		return properties{}, fmt.Errorf("alias cycle at %q", name)
	}
	p, ok := r.collection.Icons[name]
	if ok {
		if p.Body == nil || p.Parent != "" {
			return properties{}, fmt.Errorf("icon %q must have a body and no parent", name)
		}
	} else {
		p, ok = r.collection.Aliases[name]
		if !ok {
			return properties{}, fmt.Errorf("unknown icon or alias %q", name)
		}
		if p.Parent == "" || p.Body != nil {
			return properties{}, fmt.Errorf("alias %q must have a parent and no body", name)
		}
		r.active[name] = true
		parent, err := r.resolve(p.Parent, depth+1)
		delete(r.active, name)
		if err != nil {
			return properties{}, err
		}
		p = inherit(parent, p)
	}
	if p.Body == nil || len(*p.Body) == 0 || len(*p.Body) > MaxBodyBytes {
		return properties{}, fmt.Errorf("body must contain between 1 and %d bytes", MaxBodyBytes)
	}
	r.cache[name] = p
	return p, nil
}

func inherit(parent, child properties) properties {
	child.aliasDepth = parent.aliasDepth + 1
	child.dimensions = inheritDimensions(parent.dimensions, child.dimensions)
	child.Body = parent.Body
	rotation := quarterTurns(parent.Rotate) + quarterTurns(child.Rotate)
	rotation %= 4
	hFlip := value(parent.HFlip) != value(child.HFlip)
	vFlip := value(parent.VFlip) != value(child.VFlip)
	child.Rotate, child.HFlip, child.VFlip = &rotation, &hFlip, &vFlip
	return child
}

func inheritDimensions(parent, child dimensions) dimensions {
	if child.Left == nil {
		child.Left = parent.Left
	}
	if child.Top == nil {
		child.Top = parent.Top
	}
	if child.Width == nil {
		child.Width = parent.Width
	}
	if child.Height == nil {
		child.Height = parent.Height
	}
	return child
}

func quarterTurns(rotation *int) int {
	if rotation == nil {
		return 0
	}
	return ((*rotation % 4) + 4) % 4
}

func value(b *bool) bool { return b != nil && *b }

func coordinate(number *float64, fallback float64) float64 {
	if number == nil {
		return fallback
	}
	return *number
}

func (c *collection) complete(p properties) (resolvedIcon, error) {
	d := inheritDimensions(c.dimensions, p.dimensions)
	result := resolvedIcon{
		body: *p.Body,
		left: coordinate(d.Left, 0), top: coordinate(d.Top, 0),
		width: coordinate(d.Width, 16), height: coordinate(d.Height, 16),
		rotate: quarterTurns(p.Rotate), hFlip: value(p.HFlip), vFlip: value(p.VFlip),
	}
	for _, n := range []float64{result.left, result.top, result.width, result.height} {
		if math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > 1<<20 {
			return resolvedIcon{}, fmt.Errorf("viewBox coordinates must be finite and within +/-1048576")
		}
	}
	if result.width <= 0 || result.height <= 0 {
		return resolvedIcon{}, fmt.Errorf("viewBox dimensions must be positive")
	}
	return result, nil
}
