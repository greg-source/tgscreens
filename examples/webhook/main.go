// Command webhook serves the echo bot through a Telegram webhook instead of
// long polling. It needs a public HTTPS URL on port 443, 80, 88 or 8443, and a
// certificate for it: Telegram accepts a self-signed one, which Register uploads.
//
// Environment: TELEGRAM_TOKEN, WEBHOOK_URL (for example
// https://bot.example.com:8443/hook-7c1f), WEBHOOK_CERT and WEBHOOK_KEY (PEM
// file paths).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"

	"github.com/greg-source/tgscreens"
	"github.com/greg-source/tgscreens/telegram"
)

const (
	stateMain tgscreens.State  = 0
	actPing   tgscreens.Action = "ping"
)

// bot keeps users in memory. LoadUser and SaveUser are where a real bot would
// reach its database; Listen handles one update at a time, so no locking.
type bot struct {
	*tgscreens.Router

	users map[int64]tgscreens.User
}

func newBot(sender telegram.Sender) *bot {
	b := &bot{users: make(map[int64]tgscreens.User)}

	b.Router = tgscreens.New(sender, tgscreens.Config{
		LoadUser: b.loadUser,
		SaveUser: b.saveUser,

		MessageHandlers: map[tgscreens.State]tgscreens.MessageHandlerFunc{
			stateMain: b.onMessage,
		},
		CallbackHandlers: map[tgscreens.Action]tgscreens.CallbackHandlerFunc{
			actPing: b.onPing,
		},

		OnUnknownMessage:  b.onMessage,
		OnUnknownCallback: ignoreCallback,
		OnError:           logError,
		OnAction:          logAction,
	})
	return b
}

func (b *bot) loadUser(_ context.Context, username string, chatID int64) (tgscreens.User, error) {
	if user, ok := b.users[chatID]; ok {
		return user, nil
	}
	return tgscreens.User{ChatID: chatID, Username: username, State: stateMain}, nil
}

func (b *bot) saveUser(_ context.Context, user tgscreens.User) error {
	b.users[user.ChatID] = user
	return nil
}

func (b *bot) onMessage(ctx context.Context, mc tgscreens.MessageContext, user *tgscreens.User) error {
	return b.Show(ctx, user, 0, tgscreens.Screen{
		Text:    "💬 You said: " + mc.Text,
		Buttons: pingButton(1),
	})
}

func (b *bot) onPing(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	n := cc.ArgInt(0)
	return b.Show(ctx, user, cc.MessageID, tgscreens.Screen{
		Text:    fmt.Sprintf("🏓 Pong #%d", n),
		Buttons: pingButton(n + 1),
	})
}

func pingButton(n int) [][]tgscreens.Button {
	return [][]tgscreens.Button{{{Text: "🏓 Ping", Data: tgscreens.EncodeCallback(actPing, strconv.Itoa(n))}}}
}

// ignoreCallback answers a stale or unknown button quietly instead of redrawing
// the message it sits on.
func ignoreCallback(context.Context, tgscreens.CallbackContext, *tgscreens.User) error { return nil }

func logError(_ context.Context, chatID int64, _ *tgscreens.User, err error) {
	log.Printf("chat %d: handling update failed: %v", chatID, err)
}

func logAction(_ context.Context, user tgscreens.User, action string, isCallback bool, err error) {
	log.Printf("chat %d: callback=%t action=%q err=%v", user.ChatID, isCallback, action, err)
}

func main() {
	token := os.Getenv("TELEGRAM_TOKEN")
	webhookURL := os.Getenv("WEBHOOK_URL")
	certFile := os.Getenv("WEBHOOK_CERT")
	keyFile := os.Getenv("WEBHOOK_KEY")
	if token == "" || webhookURL == "" || certFile == "" || keyFile == "" {
		log.Fatal("TELEGRAM_TOKEN, WEBHOOK_URL, WEBHOOK_CERT and WEBHOOK_KEY are required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client, err := telegram.New(token)
	if err != nil {
		log.Fatal(err)
	}

	wh, err := telegram.NewWebhook(client, webhookURL, certFile, keyFile)
	if err != nil {
		log.Fatal(err)
	}
	if err := wh.Register(); err != nil {
		log.Fatal(err)
	}

	b := newBot(client)
	if err := b.Listen(ctx, wh.Listen); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
