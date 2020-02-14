package rest

import (
	"mime"
	"strings"
)

func isMimetypeBinary(t string) bool {
	m, p, err := mime.ParseMediaType(t)
	if err != nil {
		return true
	}
	if m == "application/json" {
		return false
	} else if strings.HasPrefix(m, "text/") {
		return false
	} else if _, ok := p["charset"]; ok {
		return false
	} else {
		return true
	}
}
