package generate

import (
	"fmt"
	"go/token"
	"strings"
)

var initialisms = map[string]string{
	"api": "API", "cpu": "CPU", "css": "CSS", "gpu": "GPU", "html": "HTML",
	"http": "HTTP", "https": "HTTPS", "id": "ID", "ip": "IP", "json": "JSON",
	"nfc": "NFC", "qr": "QR", "sql": "SQL", "svg": "SVG", "tv": "TV",
	"url": "URL", "usb": "USB", "xml": "XML",
}

func exportedName(name string) string {
	var result strings.Builder
	if name[0] >= '0' && name[0] <= '9' {
		result.WriteString("Icon")
	}
	previousNumeric := false
	for part := range strings.SplitSeq(name, "-") {
		numeric := part[0] >= '0' && part[0] <= '9'
		if previousNumeric && numeric {
			result.WriteByte('_')
		}
		if acronym, ok := initialisms[part]; ok {
			result.WriteString(acronym)
		} else {
			result.WriteString(strings.ToUpper(part[:1]))
			result.WriteString(part[1:])
		}
		previousNumeric = numeric
	}
	return result.String()
}

func checkPackage(name string) error {
	if !token.IsIdentifier(name) || token.Lookup(name).IsKeyword() || name == "_" || name == "main" {
		return fmt.Errorf("invalid library package name %q", name)
	}
	for i, r := range name {
		if (r < 'a' || r > 'z') && (i == 0 || r < '0' || r > '9') {
			return fmt.Errorf("package name must use lowercase ASCII letters and non-leading digits: %q", name)
		}
	}
	return nil
}
