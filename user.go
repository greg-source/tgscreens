package tgscreens

// State identifies where a user is in the conversation.
type State int

// Action identifies what an inline button does.
type Action string

// User is the per-user state the router loads, passes to handlers, and saves.
type User struct {
	ChatID   int64
	Username string
	State    State

	// MenuMessageID is the current menu message ID, or 0 when the message needs to be persisted.
	MenuMessageID int
}

// The router detects a changed User with ==, so User must stay comparable.
var _ = map[User]struct{}{}
