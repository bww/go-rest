package csrf

import (
	"testing"
	"time"

	"github.com/bww/go-util/v1/crypto"
	"github.com/stretchr/testify/assert"
)

const salt = "Salt bae"

type testData struct {
	Key   []byte
	Nonce string
	Time  time.Time
	Error error
}

type testCSRF struct {
	Input  testData
	Output testData
}

func TestCSRF(t *testing.T) {
	key1 := crypto.GenerateKey("gfnExB8lM1K84pM66bwwuLGMKTnb5sPkvdfaQ2P90n03ScB9Y9CserEURgijFkuH", salt, crypto.SHA1)
	key2 := crypto.GenerateKey("XfnExB8lM1K84pM66bwwuLGMKTnb5sPkvdfaQ2P90n03ScB9Y9CserEURgijFkuH", salt, crypto.SHA1)

	tests := []testCSRF{
		{
			Input: testData{
				Key:   key1,
				Nonce: "Hey, I'm a nonce!",
				Time:  time.Date(2019, 1, 1, 1, 0, 0, 0, time.UTC),
				Error: nil,
			},
			Output: testData{
				Key:   key1,
				Time:  time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC),
				Error: nil,
			},
		},
		{
			Input: testData{
				Key:   key1,
				Nonce: "Hey, I'm a nonce!",
				Time:  time.Date(2019, 1, 1, 1, 0, 0, 0, time.UTC),
				Error: nil,
			},
			Output: testData{
				Key:   key2,
				Time:  time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC),
				Error: ErrTokenInvalid,
			},
		},
		{
			Input: testData{
				Key:   key1,
				Nonce: "",
				Time:  time.Date(2019, 1, 1, 1, 0, 0, 0, time.UTC),
				Error: ErrNonceInsufficient,
			},
			Output: testData{
				Key:   key1,
				Time:  time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC),
				Error: nil,
			},
		},
		{
			Input: testData{
				Key:   key1,
				Nonce: "Too short.",
				Time:  time.Date(2019, 1, 1, 1, 0, 0, 0, time.UTC),
				Error: ErrNonceInsufficient,
			},
			Output: testData{
				Key:   key1,
				Time:  time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC),
				Error: nil,
			},
		},
		{
			Input: testData{
				Key:   key1,
				Nonce: "Hey, I'm a nonce!",
				Time:  time.Date(2019, 1, 1, 1, 0, 0, 0, time.UTC),
				Error: nil,
			},
			Output: testData{
				Key:   key1,
				Time:  time.Date(2019, 1, 2, 0, 0, 0, 0, time.UTC),
				Error: ErrTokenExpired,
			},
		},
	}

	for _, e := range tests {
		a, b := e.Input, e.Output
		tok, err := Sign(a.Key, CSRF{Nonce: a.Nonce, Expires: a.Time})
		assert.Equal(t, a.Error, err)
		if err == nil {
			res, err := Verify(b.Key, tok, b.Time)
			assert.Equal(t, b.Error, err)
			if err == nil {
				assert.Equal(t, a.Nonce, res.Nonce)
				assert.Equal(t, a.Time, res.Expires)
			}
		}
	}

}
