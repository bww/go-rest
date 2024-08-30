package response

import (
	"errors"
	"fmt"
	htmltempl "html/template"
	"testing"
	texttempl "text/template"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stretchr/testify/assert"
)

func TestResponseTemplate(t *testing.T) {
	tests := []struct {
		Tmpl   string
		Data   interface{}
		Expect string
		Err    func(error) error
	}{
		{
			Tmpl:   "Hello",
			Data:   struct{}{},
			Expect: "Hello",
		},
		{
			Tmpl: "Hello, {{ .User }}",
			Data: struct {
				User string
			}{
				User: "Bobbo",
			},
			Expect: "Hello, Bobbo",
		},
		{
			Tmpl: "Hello, {{ ",
			Data: struct{}{},
			Err: func(err error) error {
				if err != nil {
					return nil
				} else {
					return errors.New("Expected an error")
				}
			},
		},
		{
			Tmpl: "Hello, {{ .Nonexistent }}",
			Data: struct{}{},
			Err: func(err error) error {
				var execerr texttempl.ExecError
				if errors.As(err, &execerr) {
					return nil // exec error
				} else {
					return err
				}
			},
		},
	}
	for _, test := range tests {
		fmt.Println(">>>", test.Tmpl)
		rsp, err := HTML(test.Tmpl, test.Data)
		if test.Err != nil {
			fmt.Println("***", err)
			assert.NoError(t, test.Err(err))
		} else if assert.NoError(t, err) {
			ent, err := rsp.ReadEntity()
			if assert.NoError(t, err) {
				fmt.Println("<<<", string(ent))
				assert.Equal(t, test.Expect, string(ent))
			}
		}
	}
}

func TestResponseTemplateCache(t *testing.T) {
	var hit bool
	cache, err := lru.New[string, *htmltempl.Template](3)
	if !assert.NoError(t, err) {
		return
	}

	// nothing cached yet
	assert.Equal(t, cache.Len(), 0)

	_, hit, err = renderHTML("Hello, {{ .User }}", struct {
		User string
	}{
		User: "Jimbo",
	}, cache)
	if !assert.NoError(t, err) {
		return
	}

	// cached the first template, cache miss
	assert.Equal(t, false, hit)
	assert.Equal(t, cache.Len(), 1)

	_, hit, err = renderHTML("Hello, {{ .User }}", struct {
		User string
	}{
		User: "Jimbo",
	}, cache)
	if !assert.NoError(t, err) {
		return
	}

	// using the same template, cache  hit
	assert.Equal(t, true, hit)
	assert.Equal(t, cache.Len(), 1)

	_, hit, err = renderHTML("Hello, {{ .User }}; what's up?", struct {
		User string
	}{
		User: "Jimbo",
	}, cache)
	if !assert.NoError(t, err) {
		return
	}

	// using the a slightly different template, cache  miss, new template cached
	assert.Equal(t, false, hit)
	assert.Equal(t, cache.Len(), 2)
}
