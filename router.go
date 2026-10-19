package tgscreens

import (
	"context"
	"fmt"

	"github.com/greg-source/tgscreens/telegram"
)

// Button is an alias for telegram.Button.
type Button = telegram.Button

// Source produces updates and sends them to out.
type Source func(ctx context.Context, out chan<- telegram.Update) error

// Router dispatches Telegram updates to a bot's handlers.
type Router struct {
	tg  telegram.Sender
	cfg Config
}

// New builds a Router and panics when a required dependency is missing.
func New(tg telegram.Sender, cfg Config) *Router {
	switch {
	case tg == nil:
		panic("tgscreens: telegram.Sender is required")
	case cfg.LoadUser == nil:
		panic("tgscreens: Config.LoadUser is required")
	case cfg.SaveUser == nil:
		panic("tgscreens: Config.SaveUser is required")
	case cfg.OnUnknownMessage == nil:
		panic("tgscreens: Config.OnUnknownMessage is required")
	case cfg.OnUnknownCallback == nil:
		panic("tgscreens: Config.OnUnknownCallback is required")
	case cfg.OnError == nil:
		panic("tgscreens: Config.OnError is required")
	case cfg.OnAction == nil:
		panic("tgscreens: Config.OnAction is required")
	}

	return &Router{tg: tg, cfg: cfg}
}

// Send sends a plain message, leaving any tracked navigation message in place.
func (r *Router) Send(ctx context.Context, chatID int64, text string) error {
	if err := r.tg.SendMessage(ctx, chatID, text); err != nil {
		return fmt.Errorf("sending message to chat %d: %w", chatID, err)
	}
	return nil
}
