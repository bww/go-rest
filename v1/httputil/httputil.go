package httputil

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/bww/go-router/v1"
	"github.com/gorilla/schema"
)

var ErrUnsupportedMimetype = fmt.Errorf("Unsupported content type")

var formDecoder *schema.Decoder

func init() {
	formDecoder = schema.NewDecoder()
	formDecoder.IgnoreUnknownKeys(true)
}

// Unmarshal common entity types:
//  - application/json
//  - application/x-www-form-urlencoded
//  - multipart/form-data
func Unmarshal(req *router.Request, entity interface{}) error {
	m, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		return err
	}
	switch strings.ToLower(m) {

	case "application/json":
		err := json.NewDecoder(req.Body).Decode(entity)
		if err != nil {
			return fmt.Errorf("Could not unmarshal request entity: %v", err)
		}

	case "application/x-www-form-urlencoded", "multipart/form-data":
		err := (*http.Request)(req).ParseForm()
		if err != nil {
			return fmt.Errorf("Could not parse payload: %v", err)
		}
		err = formDecoder.Decode(entity, req.Form)
		if err != nil {
			return fmt.Errorf("Could not unmarshal request entity: %v", err)
		}

	default:
		return ErrUnsupportedMimetype

	}
	return nil
}
