package telegram

import (
	"context"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
)

func TestPoller_forwardsUpdates(t *testing.T) {
	server := newTelegramTestServer(t)
	server.updates(tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 5,
			From:      &tgbotapi.User{UserName: "greg"},
			Chat:      &tgbotapi.Chat{ID: 1},
			Text:      "hi",
		},
	})
	client := server.client(t)
	listener := server.listen(t, client)

	request := server.request(t, "getUpdates")
	assert.Equal(t, "60", request.form.Get("timeout"))
	assert.Equal(t, Update{Username: "greg", ChatID: 1, MessageID: 5, Text: "hi"}, receiveClientUpdate(t, listener.out))

	assert.ErrorIs(t, listener.stop(t), context.Canceled)
	assertClientUpdatesClosed(t, listener.out)
}

func TestPoller_acknowledgesCallbackBeforeDispatch(t *testing.T) {
	server := newTelegramTestServer(t)
	server.updates(tgbotapi.Update{
		CallbackQuery: &tgbotapi.CallbackQuery{
			ID:   "callback-id",
			From: &tgbotapi.User{UserName: "greg"},
			Message: &tgbotapi.Message{
				MessageID: 42,
				Chat:      &tgbotapi.Chat{ID: 1},
			},
			Data: "menu",
		},
	})
	client := server.client(t)
	listener := server.listen(t, client)

	request := server.request(t, "answerCallbackQuery")
	assert.Equal(t, "callback-id", request.form.Get("callback_query_id"))
	assert.Equal(t, Update{Username: "greg", ChatID: 1, MessageID: 42, Text: "menu", IsCallback: true}, receiveClientUpdate(t, listener.out))

	assert.ErrorIs(t, listener.stop(t), context.Canceled)
	assertClientUpdatesClosed(t, listener.out)
}

func TestPoller_skipsUnconvertibleUpdates(t *testing.T) {
	server := newTelegramTestServer(t)
	server.updates(
		tgbotapi.Update{UpdateID: 1},
		tgbotapi.Update{
			UpdateID: 2,
			Message: &tgbotapi.Message{
				MessageID: 5,
				From:      &tgbotapi.User{UserName: "greg"},
				Chat:      &tgbotapi.Chat{ID: 1},
				Text:      "after invalid update",
			},
		},
	)
	client := server.client(t)
	listener := server.listen(t, client)

	assert.Equal(t, Update{Username: "greg", ChatID: 1, MessageID: 5, Text: "after invalid update"}, receiveClientUpdate(t, listener.out))

	assert.ErrorIs(t, listener.stop(t), context.Canceled)
	assertClientUpdatesClosed(t, listener.out)
}

func TestPoller_stopsOnContextCancellation(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)
	listener := server.listen(t, client)

	server.request(t, "getUpdates")

	assert.ErrorIs(t, listener.stop(t), context.Canceled)
	assertClientUpdatesClosed(t, listener.out)
}
