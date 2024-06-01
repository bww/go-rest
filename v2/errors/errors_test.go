package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCauseLoop(t *testing.T) {
	defer func() {
		assert.NotNil(t, recover())
	}()
	e := New(1, "Loop", nil)
	e = e.SetCause(e)
	fmt.Println("Shouldn't get here:", e)
}
