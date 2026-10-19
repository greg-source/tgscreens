package tgscreens

import "context"

// Config is everything a bot supplies
type Config struct {
	// LoadUser and SaveUser are its only contact with storage.
	LoadUser func(ctx context.Context, username string, chatID int64) (User, error)
	SaveUser func(ctx context.Context, user User) error

	// MessageHandlers maps a state to the handler for typed text.
	MessageHandlers map[State]MessageHandlerFunc

	// CallbackHandlers maps an action to the handler for a button press.
	CallbackHandlers map[Action]CallbackHandlerFunc

	// OnUnknownMessage answers text sent in a state with no handler.
	OnUnknownMessage MessageHandlerFunc

	// OnUnknownCallback answers a button with no handler, or one pressed on a stale menu.
	OnUnknownCallback CallbackHandlerFunc

	// OnError reports a failed update; user is nil only when loading failed.
	OnError func(ctx context.Context, chatID int64, user *User, err error)

	// OnCleanupError, if set, reports a best-effort deletion that failed.
	OnCleanupError func(ctx context.Context, chatID int64, messageID int, err error)

	// OnAction reports a handled update: what the user sent — their text, or the
	// callback data the button carried — and the error it failed with, if any.
	OnAction func(ctx context.Context, user User, action string, isCallback bool, err error)
}
