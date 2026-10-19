// Command menu is a small multi-state bot: a reply keyboard for the main menu,
// a paged list of items kept in memory, hashed callback arguments, and the
// OnError, OnAction and OnCleanupError hooks. Run it with TELEGRAM_TOKEN set.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"slices"
	"strconv"

	"github.com/greg-source/tgscreens"
	"github.com/greg-source/tgscreens/telegram"
)

// In a real bot these values are persisted, so they are numbers you never reuse.
const (
	stateMenu   tgscreens.State = 0
	stateAdding tgscreens.State = 1
)

// Actions travel in callback data of buttons already delivered: append new
// ones, never rename.
const (
	actList   tgscreens.Action = "list"
	actItem   tgscreens.Action = "item"
	actDelete tgscreens.Action = "del"
)

const (
	btnItems = "Items"
	btnAdd   = "Add item"

	pageSize = 5
)

var mainKeyboard = [][]string{{btnItems, btnAdd}}

// bot keeps everything in memory. LoadUser and SaveUser are where a real bot
// would reach its database; Listen handles one update at a time, so no locking.
type bot struct {
	*tgscreens.Router

	users map[int64]tgscreens.User
	items map[int64][]string
}

func newBot(sender telegram.Sender) *bot {
	b := &bot{
		users: make(map[int64]tgscreens.User),
		items: make(map[int64][]string),
	}

	b.Router = tgscreens.New(sender, tgscreens.Config{
		LoadUser: b.loadUser,
		SaveUser: b.saveUser,

		MessageHandlers: map[tgscreens.State]tgscreens.MessageHandlerFunc{
			stateMenu:   b.onMenuMessage,
			stateAdding: b.onAddingMessage,
		},
		CallbackHandlers: map[tgscreens.Action]tgscreens.CallbackHandlerFunc{
			actList:   b.onList,
			actItem:   b.onItem,
			actDelete: b.onDelete,
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
	if user, ok := b.users[chatID]; ok {
		return user, nil
	}
	return tgscreens.User{ChatID: chatID, Username: username, State: stateMenu}, nil
}

func (b *bot) saveUser(_ context.Context, user tgscreens.User) error {
	b.users[user.ChatID] = user
	return nil
}

func (b *bot) onMenuMessage(ctx context.Context, mc tgscreens.MessageContext, user *tgscreens.User) error {
	switch mc.Text {
	case btnItems:
		return b.showList(ctx, user, 0, 0)
	case btnAdd:
		user.State = stateAdding
		return b.Keyboard(ctx, user, "Send the text of the new item.", mainKeyboard)
	default:
		return b.Keyboard(ctx, user, "Pick an option below.", mainKeyboard)
	}
}

func (b *bot) onAddingMessage(ctx context.Context, mc tgscreens.MessageContext, user *tgscreens.User) error {
	if mc.Text == "" {
		return b.Keyboard(ctx, user, "Send the item as text.", mainKeyboard)
	}

	b.items[user.ChatID] = append(b.items[user.ChatID], mc.Text)
	user.State = stateMenu
	return b.Keyboard(ctx, user, "Added: "+mc.Text, mainKeyboard)
}

func (b *bot) onList(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	return b.showList(ctx, user, cc.MessageID, cc.ArgInt(0))
}

func (b *bot) onItem(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	item, ok := tgscreens.FindByHash(b.items[user.ChatID], cc.Arg(0))
	if !ok {
		return b.showList(ctx, user, cc.MessageID, 0)
	}

	return b.Show(ctx, user, cc.MessageID, tgscreens.Screen{
		Text: item,
		Buttons: [][]tgscreens.Button{
			{{Text: "Delete", Data: tgscreens.EncodeCallback(actDelete, cc.Arg(0))}},
			{{Text: "Back", Data: tgscreens.EncodeCallback(actList, "0")}},
		},
	})
}

func (b *bot) onDelete(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	items := b.items[user.ChatID]
	if item, ok := tgscreens.FindByHash(items, cc.Arg(0)); ok {
		i := slices.Index(items, item)
		b.items[user.ChatID] = slices.Delete(items, i, i+1)
	}
	return b.showList(ctx, user, cc.MessageID, 0)
}

// showList renders one page of the chat's items. editMessageID is the message a
// pressed button sits on, or 0 when the list answers a typed message.
func (b *bot) showList(ctx context.Context, user *tgscreens.User, editMessageID, page int) error {
	items := b.items[user.ChatID]
	if len(items) == 0 {
		return b.Show(ctx, user, editMessageID, tgscreens.Screen{Text: "No items yet. Use " + btnAdd + "."})
	}

	buttons, page, totalPages := tgscreens.PagedList{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Data: func(_, _ int, item string) string {
			return tgscreens.EncodeCallback(actItem, tgscreens.Hash(item))
		},
		Nav: func(page int) string {
			return tgscreens.EncodeCallback(actList, strconv.Itoa(page))
		},
	}.Build()

	return b.Show(ctx, user, editMessageID, tgscreens.Screen{
		Text:    fmt.Sprintf("Items, page %d of %d", page+1, totalPages),
		Buttons: buttons,
	})
}

// ignoreCallback answers a stale or unknown button quietly instead of redrawing
// the message it sits on.
func ignoreCallback(context.Context, tgscreens.CallbackContext, *tgscreens.User) error { return nil }

// onError replaces navigation with a retry notice; user is nil only when
// loading the user failed, so there is no navigation to replace.
func (b *bot) onError(ctx context.Context, chatID int64, user *tgscreens.User, err error) {
	log.Printf("chat %d: handling update failed: %v", chatID, err)

	const text = "Something went wrong. Please try again."
	if user == nil {
		if sendErr := b.Send(ctx, chatID, text); sendErr != nil {
			log.Printf("chat %d: sending error message failed: %v", chatID, sendErr)
		}
		return
	}
	if showErr := b.Show(ctx, user, 0, tgscreens.Screen{Text: text}); showErr != nil {
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
	if err := b.Listen(ctx, telegram.NewPoller(client).Poll); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
