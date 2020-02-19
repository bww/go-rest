package csrf

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bww/go-util/v1/crypto"
	"github.com/bww/go-util/v1/rand"
)

const (
	sep = "$"
	min = 16
)

var (
	ErrTokenEmpty        = errors.New("CSRF token empty")
	ErrTokenMalformed    = errors.New("CSRF token malformed")
	ErrTokenInvalid      = errors.New("CSRF token invalid")
	ErrTokenExpired      = errors.New("CSRF token expired")
	ErrNonceInsufficient = fmt.Errorf("CSRF nonce must be >= %d bytes", min)
)

type Token string

type CSRF struct {
	Nonce   string    `json:"nonce"`
	Expires time.Time `json:"expires"`
}

func New(key []byte, expires time.Time) (Token, error) {
	csrf := CSRF{
		Nonce:   rand.RandomString(64),
		Expires: expires,
	}
	return Sign(key, csrf)
}

func Sign(key []byte, csrf CSRF) (Token, error) {
	if len(csrf.Nonce) < min {
		return "", ErrNonceInsufficient
	}
	enc, sig, err := crypto.SignMessage(key, crypto.SHA256, csrf)
	if err != nil {
		return "", err
	}
	return Token(sig + sep + enc), nil
}

func Verify(key []byte, token Token, now time.Time) (CSRF, error) {
	var csrf CSRF
	if token == "" {
		return csrf, ErrTokenEmpty
	}
	parts := strings.SplitN(string(token), sep, 2)
	if len(parts) != 2 {
		return csrf, ErrTokenMalformed
	}
	err := crypto.VerifyMessage(key, crypto.SHA256, parts[0], &csrf, parts[1])
	if err != nil {
		return csrf, ErrTokenInvalid
	}
	if now.After(csrf.Expires) {
		return csrf, ErrTokenExpired
	}
	if len(csrf.Nonce) < min {
		return csrf, ErrNonceInsufficient
	}
	return csrf, nil
}
