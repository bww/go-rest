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

// default maximum memory per request
const memMax = 1 << 25

var ErrUnsupportedMimetype = fmt.Errorf("Unsupported content type")

var formDecoder *schema.Decoder

func init() {
	formDecoder = schema.NewDecoder()
	formDecoder.IgnoreUnknownKeys(true)
}

// Unmarshal requests with common entity types
func Unmarshal(req *router.Request, entity interface{}, opts ...Option) error {
	return UnmarshalWithConfig(req, entity, Config{
		MaxMem: maxMem,
	}.WithOptions(opts...))
}

// Unmarshal requests with common entity types:
//   - application/json
//   - application/x-www-form-urlencoded
//   - multipart/form-data
func UnmarshalWithConfig(req *router.Request, entity interface{}, conf Config) error {
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

	case "multipart/form-data":
		err := (*http.Request)(req).ParseMultipartForm(conf.MaxMem)
		if err != nil {
			return fmt.Errorf("Could not parse multipart form: %v", err)
		}
		err = formDecoder.Decode(entity, req.Form) // caller must access multipart data separately
		if err != nil {
			return fmt.Errorf("Could not unmarshal request entity: %v", err)
		}

	case "application/x-www-form-urlencoded":
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
