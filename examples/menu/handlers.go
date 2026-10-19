package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/greg-source/tgscreens"
)

const maxTextRunes = 200

// Handlers change the store and the user, then show a screen. They never save
// the user: the router does that when the user changed.

// onMenuMessage answers the reply keyboard. The typed message is deleted, so
// the chat shows only the current screen.
func (b *bot) onMenuMessage(ctx context.Context, mc tgscreens.MessageContext, user *tgscreens.User) error {
	b.Discard(ctx, user.ChatID, mc.MessageID)

	switch mc.Text {
	case btnNew:
		user.State = stateWriting
		return b.Show(ctx, user, 0, writeScreen(""))
	case btnList:
		return b.showList(ctx, user, 0, 0, "")
	case btnSettings:
		return b.Show(ctx, user, 0, settingsScreen(b.store.isPaused(user.ChatID), ""))
	default:
		return b.Keyboard(ctx, user, txtWelcome, mainKeyboard)
	}
}

// onWritingMessage takes the text of a new reminder and asks when to send it.
// A reply-keyboard caption means the user went elsewhere instead.
func (b *bot) onWritingMessage(ctx context.Context, mc tgscreens.MessageContext, user *tgscreens.User) error {
	if mc.Text == cmdStart || slices.ContainsFunc(mainKeyboard, func(row []string) bool { return slices.Contains(row, mc.Text) }) {
		user.State = stateMenu
		return b.onMenuMessage(ctx, mc, user)
	}
	b.Discard(ctx, user.ChatID, mc.MessageID)

	text := strings.TrimSpace(mc.Text)
	switch {
	case text == "":
		return b.Show(ctx, user, 0, writeScreen(txtTextEmpty))
	case utf8.RuneCountInString(text) > maxTextRunes:
		return b.Show(ctx, user, 0, writeScreen(txtTextTooLong))
	}

	b.store.setDraft(user.ChatID, text)
	user.State = stateMenu
	return b.Show(ctx, user, 0, whenScreen(text))
}

func (b *bot) onNew(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	user.State = stateWriting
	return b.Show(ctx, user, cc.MessageID, writeScreen(""))
}

func (b *bot) onWhen(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	text, ok := b.store.takeDraft(user.ChatID)
	if !ok {
		return b.showList(ctx, user, cc.MessageID, 0, txtDraftLost)
	}

	due := b.now().Add(time.Duration(cc.ArgInt(0)) * time.Minute)
	r := b.store.add(user.ChatID, text, due)
	return b.Show(ctx, user, cc.MessageID, reminderSetScreen(r))
}

// onCancel is the only button while writing, so it is the one callback that
// has to end writing. On the time screen it drops the draft instead.
func (b *bot) onCancel(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	user.State = stateMenu
	b.store.takeDraft(user.ChatID)
	return b.showList(ctx, user, cc.MessageID, 0, "")
}

func (b *bot) onList(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	return b.showList(ctx, user, cc.MessageID, cc.ArgInt(0), "")
}

// onReminder opens one reminder. It may have been delivered or deleted since
// the list was drawn, so a miss shows the list again instead.
func (b *bot) onReminder(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	page := cc.ArgInt(1)
	r, ok := b.store.find(user.ChatID, cc.ArgInt(0))
	if !ok {
		return b.showList(ctx, user, cc.MessageID, page, txtReminderGone)
	}
	return b.Show(ctx, user, cc.MessageID, reminderScreen(r, page))
}

func (b *bot) onDelete(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	status := txtReminderGone
	if r, ok := b.store.remove(user.ChatID, cc.ArgInt(0)); ok {
		status = fmt.Sprintf(txtDeleted, r.Text)
	}
	return b.showList(ctx, user, cc.MessageID, cc.ArgInt(1), status)
}

func (b *bot) onSettings(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	return b.Show(ctx, user, cc.MessageID, settingsScreen(b.store.isPaused(user.ChatID), ""))
}

func (b *bot) onPause(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	paused := cc.Arg(0) == "1"
	b.store.setPaused(user.ChatID, paused)
	return b.Show(ctx, user, cc.MessageID, settingsScreen(paused, ""))
}

func (b *bot) onClear(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	return b.Show(ctx, user, cc.MessageID, clearScreen())
}

func (b *bot) onClearOK(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	b.store.clear(user.ChatID)
	return b.Show(ctx, user, cc.MessageID, settingsScreen(b.store.isPaused(user.ChatID), txtAllDeleted))
}

// showList renders one page of the chat's reminders. editMessageID is the
// message a pressed button sits on, or 0 when the list answers a typed message.
func (b *bot) showList(ctx context.Context, user *tgscreens.User, editMessageID, page int, status string) error {
	screen := listScreen(b.store.list(user.ChatID), b.store.isPaused(user.ChatID), status, page)
	return b.Show(ctx, user, editMessageID, screen)
}

// deliverDue sends every reminder due at now as a plain message, outside
// navigation, so it does not replace the screen the user is on.
func (b *bot) deliverDue(ctx context.Context, now time.Time) {
	for _, d := range b.store.takeDue(now) {
		if err := b.Send(ctx, d.chatID, fmt.Sprintf(txtDelivery, d.Text)); err != nil {
			log.Printf("chat %d: delivering reminder %d failed: %v", d.chatID, d.ID, err)
		}
	}
}

// remind delivers due reminders every tick until ctx is cancelled.
func (b *bot) remind(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			b.deliverDue(ctx, now)
		}
	}
}
