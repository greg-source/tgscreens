package tgscreens

import (
	"context"
	"strconv"
)

// MessageContext carries a text update to a message handler.
type MessageContext struct {
	MessageID int
	Text      string
}

// CallbackContext carries a button press to a callback handler.
type CallbackContext struct {
	MessageID int
	Action    Action
	Args      []string
}

// Arg returns the i-th callback argument, or "" if there is none.
func (c CallbackContext) Arg(i int) string {
	if i < 0 || i >= len(c.Args) {
		return ""
	}
	return c.Args[i]
}

// ArgInt returns the i-th callback argument as an int, or 0 if it is missing or
// not a number.
func (c CallbackContext) ArgInt(i int) int {
	n, err := strconv.Atoi(c.Arg(i))
	if err != nil {
		return 0
	}
	return n
}

// MessageHandlerFunc handles a text message from a user in a given state.
//
// Mutating user is how a handler changes state or moves the conversation on;
// the router persists it after the handler returns, so handlers never save.
type MessageHandlerFunc func(ctx context.Context, mc MessageContext, user *User) error

// CallbackHandlerFunc handles an inline button press. The same rule about
// mutating user applies.
type CallbackHandlerFunc func(ctx context.Context, cc CallbackContext, user *User) error
