package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jcamilovallejos/linkguard/internal/usecase"
)

func TestRandomCodeGenerator_Generate(t *testing.T) {
	gen := usecase.NewRandomCodeGenerator()

	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		code, err := gen.Generate()
		require.NoError(t, err)
		assert.Len(t, code, 7)
		assert.False(t, seen[code], "generated a duplicate code within a small sample")
		seen[code] = true

		for _, r := range code {
			assert.True(t, (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'), "code %q contains a non-base62 character", code)
		}
	}
}
