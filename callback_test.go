package tgscreens

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCallback_roundTrip(t *testing.T) {
	tests := []struct {
		name     string
		action   Action
		args     []string
		wantData string
	}{
		{name: "no arguments", action: "menu", args: nil, wantData: "menu"},
		{name: "one argument", action: "new", args: []string{"2"}, wantData: "new:2"},
		{name: "two arguments", action: "event", args: []string{"a3f9c012", "7b21ee40"}, wantData: "event:a3f9c012|7b21ee40"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := EncodeCallback(tt.action, tt.args...)
			assert.Equal(t, tt.wantData, data)

			action, args := decodeCallback(data)
			assert.Equal(t, tt.action, action)
			assert.Equal(t, tt.args, args)
		})
	}
}

// Garbage must land on an action no handler is registered for, so Route sends
// it to OnUnknownCallback rather than to whichever handler sits at a zero value.
func TestDecodeCallback_malformedInput(t *testing.T) {
	tests := []struct {
		name       string
		data       string
		wantAction Action
		wantArgs   []string
	}{
		{name: "empty", data: "", wantAction: "", wantArgs: nil},
		{name: "no separator", data: "garbage", wantAction: "garbage", wantArgs: nil},
		{name: "trailing separator", data: "menu:", wantAction: "menu", wantArgs: []string{""}},
		{name: "legacy integer action", data: "3", wantAction: "3", wantArgs: nil},
		{name: "legacy integer with param", data: "3:Some name.", wantAction: "3", wantArgs: []string{"Some name."}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, args := decodeCallback(tt.data)
			assert.Equal(t, tt.wantAction, action)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

// Exceeding the cap fails the entire keyboard, not the offending button, so the
// worst realistic callback has to be measured rather than assumed.
func TestCallback_fitsTelegramBudget(t *testing.T) {
	longestGroup := "A group name long enough to overflow the callback budget if it were sent raw"
	longestItem := "An item name that is also long enough to overflow the budget if sent raw"

	tests := map[string]string{
		"menu":       EncodeCallback("menu"),
		"paged list": EncodeCallback("new", "999"),
		"group page": EncodeCallback("group", Hash(longestGroup), "999"),
		"item":       EncodeCallback("item", Hash(longestGroup), Hash(longestItem)),
		"delete":     EncodeCallback("delok", Hash(longestGroup)),
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			assert.LessOrEqual(t, len(data), MaxCallbackDataBytes, "callback %q is %d bytes", data, len(data))
		})
	}
}
