// Command menu is a reminder bot: a reply keyboard for the main menu, a
// writing state that captures text, inline buttons that pick a time, a paged
// list of reminders, and a settings screen with an on/off toggle and a
// confirmed delete. Everything is kept in memory. Run it with TELEGRAM_TOKEN set.
//
// main.go wires the bot; handlers.go reacts to updates, screens.go builds what
// is shown, text.go holds the copy, and store.go is the in-memory storage.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/greg-source/tgscreens"
	"github.com/greg-source/tgscreens/telegram"
)

// In a real bot these values are persisted, so they are numbers you never reuse.
const (
	stateMenu    tgscreens.State = 0
	stateWriting tgscreens.State = 1
)

// Actions travel in callback data of buttons already delivered: append new
// ones, never rename.
const (
	actNew      tgscreens.Action = "new"
	actWhen     tgscreens.Action = "when"
	actCancel   tgscreens.Action = "cancel"
	actList     tgscreens.Action = "list"
	actReminder tgscreens.Action = "rem"
	actDelete   tgscreens.Action = "del"
	actSettings tgscreens.Action = "set"
	actPause    tgscreens.Action = "pause"
	actClear    tgscreens.Action = "clear"
	actClearOK  tgscreens.Action = "clearok"
)

type bot struct {
	*tgscreens.Router

	store *store
	now   func() time.Time
}

func newBot(sender telegram.Sender) *bot {
	b := &bot{store: newStore(), now: time.Now}

	b.Router = tgscreens.New(sender, tgscreens.Config{
		LoadUser: b.loadUser,
		SaveUser: b.saveUser,

		MessageHandlers: map[tgscreens.State]tgscreens.MessageHandlerFunc{
			stateMenu:    b.onMenuMessage,
			stateWriting: b.onWritingMessage,
		},
		CallbackHandlers: map[tgscreens.Action]tgscreens.CallbackHandlerFunc{
			actNew:      b.onNew,
			actWhen:     b.onWhen,
			actCancel:   b.onCancel,
			actList:     b.onList,
			actReminder: b.onReminder,
			actDelete:   b.onDelete,
			actSettings: b.onSettings,
			actPause:    b.onPause,
			actClear:    b.onClear,
			actClearOK:  b.onClearOK,
		},

		OnUnknownMessage:  b.onMenuMessage,
		OnUnknownCallback: ignoreCallback,
		OnError:           b.onError,
		OnAction:          logAction,
		OnCleanupError:    logCleanupError,
	})
	return b
}

func (b *bot) loadUser(_ context.Context, username string, chatID int64) (tgscreens.User, error) {
	if user, ok := b.store.user(chatID); ok {
		return user, nil
	}
	return tgscreens.User{ChatID: chatID, Username: username, State: stateMenu}, nil
}

func (b *bot) saveUser(_ context.Context, user tgscreens.User) error {
	b.store.saveUser(user)
	return nil
}

// ignoreCallback answers a stale or unknown button quietly instead of redrawing
// the message it sits on.
func ignoreCallback(context.Context, tgscreens.CallbackContext, *tgscreens.User) error { return nil }

// onError replaces navigation with a retry notice; user is nil only when
// loading the user failed, so there is no navigation to replace.
func (b *bot) onError(ctx context.Context, chatID int64, user *tgscreens.User, err error) {
	log.Printf("chat %d: handling update failed: %v", chatID, err)

	if user == nil {
		if sendErr := b.Send(ctx, chatID, txtError); sendErr != nil {
			log.Printf("chat %d: sending error message failed: %v", chatID, sendErr)
		}
		return
	}
	if showErr := b.Show(ctx, user, 0, tgscreens.Screen{Text: txtError}); showErr != nil {
		log.Printf("chat %d: showing error screen failed: %v", chatID, showErr)
	}
}

func logAction(_ context.Context, user tgscreens.User, action string, isCallback bool, err error) {
	log.Printf("chat %d: callback=%t action=%q err=%v", user.ChatID, isCallback, action, err)
}

func logCleanupError(_ context.Context, chatID int64, messageID int, err error) {
	log.Printf("chat %d: deleting message %d failed: %v", chatID, messageID, err)
}

func main() {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_TOKEN is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client, err := telegram.New(token)
	if err != nil {
		log.Fatal(err)
	}
	// Long polling fails with a conflict while a webhook is registered.
	if err := client.DeleteWebhook(ctx); err != nil {
		log.Fatal(err)
	}

	b := newBot(client)
	go b.remind(ctx, 5*time.Second)

	if err := b.Listen(ctx, telegram.NewPoller(client).Poll); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
