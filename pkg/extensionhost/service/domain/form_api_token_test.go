package servicedomain

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAndHashFormAPIToken(t *testing.T) {
	first, err := GenerateFormAPIToken()
	require.NoError(t, err)
	second, err := GenerateFormAPIToken()
	require.NoError(t, err)

	assert.Regexp(t, regexp.MustCompile(`^fat_[0-9a-f]{64}$`), first)
	assert.NotEqual(t, first, second)
	assert.Len(t, HashFormAPIToken(first), 64)
	assert.NotEqual(t, first, HashFormAPIToken(first))
	assert.Equal(t, HashFormAPIToken(first), HashFormAPIToken(first))
}
