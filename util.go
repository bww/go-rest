package rest

import (
	"mime"
	"strings"
)

func isMimetypePrintable(t string) bool {
	m, p, err := mime.ParseMediaType(t)
	if err != nil {
		return false // assume not
	}
	switch {
	case m == "application/json":
		return true
	case m == "application/x-www-form-urlencoded":
		return true
	case strings.HasPrefix(m, "text/"):
		return true
	case p["charset"] != "": // if a charset is specified, it's printable
		return true
	default:
		return false
	}
}
