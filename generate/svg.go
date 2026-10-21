package generate

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

func number(n float64) string {
	if n == 0 {
		return "0"
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

func (icon resolvedIcon) svg() (string, string, error) {
	box := [4]float64{icon.left, icon.top, icon.width, icon.height}
	var transforms []string
	rotation := icon.rotate
	if icon.hFlip && icon.vFlip {
		rotation += 2
	} else if icon.hFlip {
		transforms = append(transforms, "translate("+number(box[2]+box[0])+" "+number(-box[1])+")", "scale(-1 1)")
		box[0], box[1] = 0, 0
	} else if icon.vFlip {
		transforms = append(transforms, "translate("+number(-box[0])+" "+number(box[3]+box[1])+")", "scale(1 -1)")
		box[0], box[1] = 0, 0
	}
	rotation %= 4
	var turn string
	switch rotation {
	case 1:
		center := number(box[3]/2 + box[1])
		turn = "rotate(90 " + center + " " + center + ")"
	case 2:
		turn = "rotate(180 " + number(box[2]/2+box[0]) + " " + number(box[3]/2+box[1]) + ")"
	case 3:
		center := number(box[2]/2 + box[0])
		turn = "rotate(-90 " + center + " " + center + ")"
	}
	if turn != "" {
		transforms = append([]string{turn}, transforms...)
	}
	if rotation%2 == 1 {
		box[0], box[1], box[2], box[3] = box[1], box[0], box[3], box[2]
	}
	body := icon.body
	if len(transforms) != 0 {
		if err := validateSVG(`<svg xmlns="http://www.w3.org/2000/svg">` + body + `</svg>`); err != nil {
			return "", "", err
		}
		defs, content, err := splitDefs(body)
		if err != nil {
			return "", "", err
		}
		body = defs + `<g transform="` + strings.Join(transforms, " ") + `">` + content + `</g>`
	}
	viewBox := number(box[0]) + " " + number(box[1]) + " " + number(box[2]) + " " + number(box[3])
	document := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="` + viewBox + `">` + body + `</svg>`
	if len(document) > MaxBodyBytes+4096 {
		return "", "", fmt.Errorf("normalized SVG exceeds size budget")
	}
	if err := validateSVG(document); err != nil {
		return "", "", err
	}
	return document, viewBox, nil
}

// Keep definitions outside the transformation group, as Iconify's builder does.
func splitDefs(body string) (string, string, error) {
	decoder := xml.NewDecoder(strings.NewReader(body))
	var defs, content strings.Builder
	cursor := int64(0)
	for {
		start := decoder.InputOffset()
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", fmt.Errorf("decode SVG body: %w", err)
		}
		element, ok := token.(xml.StartElement)
		if !ok || element.Name.Local != "defs" {
			continue
		}
		innerStart := decoder.InputOffset()
		if err := decoder.Skip(); err != nil {
			return "", "", fmt.Errorf("decode SVG definitions: %w", err)
		}
		end := decoder.InputOffset()
		innerEnd := int64(strings.LastIndex(body[:end], "<"))
		content.WriteString(strings.TrimSpace(body[cursor:start]))
		if innerEnd >= innerStart {
			defs.WriteString(strings.TrimSpace(body[innerStart:innerEnd]))
		}
		cursor = end
	}
	content.WriteString(body[cursor:])
	if defs.Len() == 0 {
		return "", content.String(), nil
	}
	return "<defs>" + defs.String() + "</defs>", content.String(), nil
}

var supportedElements = map[string]bool{
	"svg": true, "g": true, "a": true, "switch": true, "use": true,
	"symbol": true, "path": true, "rect": true, "circle": true, "ellipse": true,
	"line": true, "polyline": true, "polygon": true, "defs": true,
	"linearGradient": true, "radialGradient": true, "stop": true,
	"clipPath": true, "mask": true, "title": true, "desc": true, "metadata": true,
}

var styleProperties = map[string]bool{
	"fill": true, "stroke": true, "fill-opacity": true, "stroke-opacity": true,
	"opacity": true, "stop-opacity": true, "fill-rule": true, "clip-rule": true,
	"stroke-width": true, "stroke-miterlimit": true, "stroke-linecap": true,
	"stroke-linejoin": true, "stroke-dasharray": true, "stroke-dashoffset": true,
	"color": true, "display": true, "visibility": true, "paint-order": true,
	"clip-path": true, "mask": true, "mask-type": true, "stop-color": true,
}

func validateSVG(document string) error {
	decoder := xml.NewDecoder(strings.NewReader(document))
	depth, tokens, roots, textDepth := 0, 0, 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid SVG: %w", err)
		}
		tokens++
		if tokens > 100_000 {
			return fmt.Errorf("SVG exceeds 100000 XML tokens")
		}
		switch element := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots > 1 || element.Name.Local != "svg" {
					return fmt.Errorf("SVG must contain exactly one svg root")
				}
			}
			depth++
			if depth > 128 {
				return fmt.Errorf("SVG exceeds 128 nested elements")
			}
			if textDepth == 0 && (element.Name.Local == "title" || element.Name.Local == "desc" || element.Name.Local == "metadata") {
				textDepth = depth
			}
			if !supportedElements[element.Name.Local] ||
				(element.Name.Space != "" && element.Name.Space != "http://www.w3.org/2000/svg") {
				return fmt.Errorf("SVG element <%s> is unsupported by the MyGo target", element.Name.Local)
			}
			for _, attribute := range element.Attr {
				if err := validateAttribute(attribute); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if depth == textDepth {
				textDepth = 0
			}
			depth--
		case xml.CharData:
			if textDepth == 0 && strings.TrimSpace(string(element)) != "" {
				return fmt.Errorf("SVG text content is unsupported; convert text to paths")
			}
		case xml.Directive, xml.ProcInst:
			return fmt.Errorf("SVG directives and processing instructions are unsupported")
		}
	}
	if roots != 1 || depth != 0 {
		return fmt.Errorf("SVG must contain one complete root")
	}
	if _, err := ui.ParseSVG([]byte(document)); err != nil {
		return fmt.Errorf("MyGo SVG parse: %w", err)
	}
	return nil
}

func validateAttribute(attribute xml.Attr) error {
	name, value := attribute.Name.Local, strings.TrimSpace(attribute.Value)
	if name == "d" {
		return validatePath(value)
	}
	if strings.HasPrefix(strings.ToLower(name), "on") || name == "filter" ||
		strings.HasPrefix(name, "marker-") || name == "vector-effect" {
		return fmt.Errorf("SVG attribute %q is unsupported by the MyGo target", name)
	}
	if name == "href" && !strings.HasPrefix(value, "#") {
		return fmt.Errorf("SVG external reference %q is unsupported", value)
	}
	if strings.Contains(strings.ToLower(value), "url(") {
		remaining := value
		for {
			index := strings.Index(strings.ToLower(remaining), "url(")
			if index < 0 {
				break
			}
			remaining = remaining[index+4:]
			end := strings.IndexByte(remaining, ')')
			if end < 0 || !strings.HasPrefix(strings.Trim(remaining[:end], " \t\r\n'\""), "#") {
				return fmt.Errorf("SVG paint reference must use a local fragment")
			}
			remaining = remaining[end+1:]
		}
	}
	if name == "style" {
		for declaration := range strings.SplitSeq(value, ";") {
			if strings.TrimSpace(declaration) == "" {
				continue
			}
			property, setting, ok := strings.Cut(declaration, ":")
			if !ok || !styleProperties[strings.TrimSpace(property)] || strings.Contains(setting, "!important") {
				return fmt.Errorf("unsupported SVG style declaration %q", declaration)
			}
		}
	}
	return nil
}
