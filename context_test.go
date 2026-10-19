package tgscreens

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCallbackContext_argReturnsEmptyForMissingIndexes(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		index int
		want  string
	}{
		{name: "present", args: []string{"first", "second"}, index: 1, want: "second"},
		{name: "out of range", args: []string{"first"}, index: 1, want: ""},
		{name: "negative", args: []string{"first"}, index: -1, want: ""},
		{name: "empty arguments", index: 0, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callback := CallbackContext{Args: tt.args}
			var got string

			assert.NotPanics(t, func() { got = callback.Arg(tt.index) })
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCallbackContext_argIntReturnsZeroForInvalidIndexesAndValues(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		index int
		want  int
	}{
		{name: "number", args: []string{"2"}, index: 0, want: 2},
		{name: "non-numeric", args: []string{"two"}, index: 0, want: 0},
		{name: "out of range", args: []string{"2"}, index: 1, want: 0},
		{name: "negative", args: []string{"2"}, index: -1, want: 0},
		{name: "empty arguments", index: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callback := CallbackContext{Args: tt.args}
			var got int

			assert.NotPanics(t, func() { got = callback.ArgInt(tt.index) })
			assert.Equal(t, tt.want, got)
		})
	}
}
