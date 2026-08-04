package usecase

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const (
	codeAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	codeLength   = 7
)

// RandomCodeGenerator generates short codes as random base62 strings using
// a cryptographically secure source, so codes are not guessable. It has no
// external dependencies, which is why it lives here rather than under
// adapter: there is nothing to swap out in tests.
type RandomCodeGenerator struct{}

// NewRandomCodeGenerator returns the default domain.CodeGenerator.
func NewRandomCodeGenerator() RandomCodeGenerator {
	return RandomCodeGenerator{}
}

// Generate returns a new random short code candidate.
func (RandomCodeGenerator) Generate() (string, error) {
	alphabetSize := big.NewInt(int64(len(codeAlphabet)))
	code := make([]byte, codeLength)
	for i := range code {
		n, err := rand.Int(rand.Reader, alphabetSize)
		if err != nil {
			return "", fmt.Errorf("generate short code: %w", err)
		}
		code[i] = codeAlphabet[n.Int64()]
	}
	return string(code), nil
}
