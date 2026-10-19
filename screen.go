package tgscreens

import (
	"context"
	"fmt"

	"github.com/greg-source/tgscreens/telegram"
)

// Screen is a message with an inline keyboard.
type Screen struct {
	Text string

	// ParseMode controls Telegram's formatting of Text; the zero value sends plain text.
	ParseMode telegram.ParseMode
	Buttons   [][]Button
}

// Show edits message editMessageID in place, or when it is 0 sends the screen as the new tracked navigation message.
func (r *Router) Show(ctx context.Context, user *User, editMessageID int, s Screen) error {
	if editMessageID != 0 {
		if err := r.tg.EditMessageWithButtons(ctx, user.ChatID, editMessageID, s.Text, s.Buttons, s.ParseMode); err != nil {
			return fmt.Errorf("editing screen %d in chat %d: %w", editMessageID, user.ChatID, err)
		}
		return nil
	}

	// Sending a second menu would strand the one already on screen.
	r.dropMenu(ctx, user)

	id, err := r.tg.SendMessageWithButtons(ctx, user.ChatID, s.Text, s.Buttons, s.ParseMode)
	if err != nil {
		return fmt.Errorf("sending screen to chat %d: %w", user.ChatID, err)
	}
	user.MenuMessageID = id
	return nil
}

// Keyboard deletes the tracked navigation message (user.MenuMessageID), then sends a reply keyboard.
func (r *Router) Keyboard(ctx context.Context, user *User, text string, keyboard [][]string) error {
	r.dropMenu(ctx, user)
	return r.SendKeyboard(ctx, user, text, keyboard)
}

// SendKeyboard sends a reply keyboard and leaves the tracked navigation message alone.
func (r *Router) SendKeyboard(ctx context.Context, user *User, text string, keyboard [][]string) error {
	if err := r.tg.SendMessageWithKeyboard(ctx, user.ChatID, text, keyboard); err != nil {
		return fmt.Errorf("sending keyboard to chat %d: %w", user.ChatID, err)
	}
	return nil
}

// Discard deletes an incoming message, best effort, and leaves navigation alone.
func (r *Router) Discard(ctx context.Context, chatID int64, messageID int) {
	if messageID == 0 {
		return
	}
	if err := r.tg.DeleteMessage(ctx, chatID, messageID); err != nil {
		r.cleanupFailed(ctx, chatID, messageID, err)
	}
}

// dropMenu deletes the tracked navigation message before forgetting its ID.
func (r *Router) dropMenu(ctx context.Context, user *User) {
	if user.MenuMessageID == 0 {
		return
	}

	if err := r.tg.DeleteMessage(ctx, user.ChatID, user.MenuMessageID); err != nil {
		r.cleanupFailed(ctx, user.ChatID, user.MenuMessageID, err)
	}
	user.MenuMessageID = 0
}

// cleanupFailed reports a failed best-effort deletion when a hook is set.
func (r *Router) cleanupFailed(ctx context.Context, chatID int64, messageID int, err error) {
	if r.cfg.OnCleanupError != nil {
		r.cfg.OnCleanupError(ctx, chatID, messageID, err)
	}
}
