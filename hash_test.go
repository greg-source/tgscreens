package tgscreens

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHash_isShortAndStable(t *testing.T) {
	const name = "First name."

	assert.Len(t, Hash(name), 8)
	assert.Equal(t, Hash(name), Hash(name))
	assert.NotEqual(t, Hash(name), Hash("Second name."))
}

func TestFindByHash(t *testing.T) {
	items := []string{"First name.", "Second name", "Third name"}

	got, ok := FindByHash(items, Hash("Second name"))
	require.True(t, ok)
	assert.Equal(t, "Second name", got)

	_, ok = FindByHash(items, "deadbeef")
	assert.False(t, ok, "a hash that no longer resolves must report miss, not a wrong item")
}
