package tgscreens

import (
	"strings"
)

// MaxCallbackDataBytes is Telegram's cap on callback data.
const MaxCallbackDataBytes = 64

const (
	actionSep = ":"
	argSep    = "|"
)

// EncodeCallback encodes an action and its arguments as "action:arg|arg".
//
// Arguments must be short opaque values, such as a Hash or a small integer,
// never user text. Free text blows the MaxCallbackDataBytes budget and corrupts
// the encoding if it happens to contain a separator.
func EncodeCallback(action Action, args ...string) string {
	if len(args) == 0 {
		return string(action)
	}
	return string(action) + actionSep + strings.Join(args, argSep)
}

// decodeCallback splits data produced by EncodeCallback. Malformed or unrecognised
// data yields an action matching no handler, which dispatch sends to
// Config.OnUnknownCallback.
func decodeCallback(data string) (Action, []string) {
	action, rest, ok := strings.Cut(data, actionSep)
	if !ok {
		return Action(data), nil
	}
	return Action(action), strings.Split(rest, argSep)
}
