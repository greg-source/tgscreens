package telegram

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_sendsMessage(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)

	require.NoError(t, client.SendMessage(context.Background(), 42, "hello"))

	request := server.request(t, "sendMessage")
	assert.Equal(t, "42", request.form.Get("chat_id"))
	assert.Equal(t, "hello", request.form.Get("text"))
}

func TestClient_sendsMessageReturnsTelegramError(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)
	server.fail("sendMessage", "telegram down")

	err := client.SendMessage(context.Background(), 42, "hello")

	require.Error(t, err)
	assert.ErrorContains(t, err, "sending telegram message to chat 42")
	assert.ErrorContains(t, err, "telegram down")
}

func TestClient_sendsMessageWithButtons(t *testing.T) {
	buttons := [][]Button{
		{{Text: "first", Data: "one"}, {Text: "second", Data: "two"}},
		{{Text: "back", Data: "menu"}},
	}
	server := newTelegramTestServer(t)
	server.respond("sendMessage", map[string]any{"message_id": 17})
	client := server.client(t)

	messageID, err := client.SendMessageWithButtons(context.Background(), 42, "pick one", buttons, ParseModePlain)

	require.NoError(t, err)
	assert.Equal(t, 17, messageID)
	request := server.request(t, "sendMessage")
	assert.Equal(t, "42", request.form.Get("chat_id"))
	assert.Equal(t, "pick one", request.form.Get("text"))
	assert.Empty(t, request.form.Get("parse_mode"))
	assertInlineKeyboard(t, request.form.Get("reply_markup"), buttons)
}

func TestClient_sendsMessageWithButtonsUsesHTMLParseMode(t *testing.T) {
	server := newTelegramTestServer(t)
	server.respond("sendMessage", map[string]any{"message_id": 17})
	client := server.client(t)

	_, err := client.SendMessageWithButtons(context.Background(), 42, "<b>pick one</b>", nil, ParseModeHTML)

	require.NoError(t, err)
	assert.Equal(t, "HTML", server.request(t, "sendMessage").form.Get("parse_mode"))
}

func TestClient_sendsMessageWithButtonsReturnsTelegramError(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)
	server.fail("sendMessage", "telegram down")

	messageID, err := client.SendMessageWithButtons(context.Background(), 42, "pick one", [][]Button{{{Text: "one", Data: "one"}}}, ParseModePlain)

	assert.Zero(t, messageID)
	require.Error(t, err)
	assert.ErrorContains(t, err, "sending telegram message with buttons to chat 42")
	assert.ErrorContains(t, err, "telegram down")
}

func TestClient_sendsMessageWithNoButtonsLeavesReplyMarkupUnset(t *testing.T) {
	server := newTelegramTestServer(t)
	server.respond("sendMessage", map[string]any{"message_id": 18})
	client := server.client(t)

	messageID, err := client.SendMessageWithButtons(context.Background(), 42, "pick one", nil, ParseModePlain)

	require.NoError(t, err)
	assert.Equal(t, 18, messageID)
	assert.Empty(t, server.request(t, "sendMessage").form.Get("reply_markup"))
}

func TestClient_editsMessageWithButtons(t *testing.T) {
	buttons := [][]Button{{{Text: "back", Data: "menu"}}}
	server := newTelegramTestServer(t)
	client := server.client(t)

	require.NoError(t, client.EditMessageWithButtons(context.Background(), 42, 17, "updated", buttons, ParseModePlain))

	request := server.request(t, "editMessageText")
	assert.Equal(t, "42", request.form.Get("chat_id"))
	assert.Equal(t, "17", request.form.Get("message_id"))
	assert.Equal(t, "updated", request.form.Get("text"))
	assert.Empty(t, request.form.Get("parse_mode"))
	assertInlineKeyboard(t, request.form.Get("reply_markup"), buttons)
}

func TestClient_editsMessageWithButtonsUsesHTMLParseMode(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)

	err := client.EditMessageWithButtons(context.Background(), 42, 17, "<b>updated</b>", nil, ParseModeHTML)

	require.NoError(t, err)
	assert.Equal(t, "HTML", server.request(t, "editMessageText").form.Get("parse_mode"))
}

// A nil-slice reply markup marshals to JSON null, which the Bot API rejects
// for editMessageText with "field \"inline_keyboard\" must be of type Array".
// Editing a screen down to no buttons — the farewell after a data deletion,
// for instance — must still clear the previous keyboard by sending an empty
// Array.
func TestClient_editingMessageWithNoButtonsSendsAnEmptyArrayNotNull(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)

	require.NoError(t, client.EditMessageWithButtons(context.Background(), 42, 17, "done", nil, ParseModePlain))

	request := server.request(t, "editMessageText")
	var markup struct {
		InlineKeyboard *[][]struct{} `json:"inline_keyboard"`
	}
	require.NoError(t, json.Unmarshal([]byte(request.form.Get("reply_markup")), &markup))
	require.NotNil(t, markup.InlineKeyboard, "inline_keyboard must be an empty array, not null")
	assert.Empty(t, *markup.InlineKeyboard)
}

func TestClient_editingUnchangedMessageSucceeds(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)
	server.fail("editMessageText", "message is not modified")

	err := client.EditMessageWithButtons(context.Background(), 42, 17, "updated", nil, ParseModePlain)

	require.NoError(t, err)
}

func TestClient_editingMessageReturnsOtherTelegramError(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)
	server.fail("editMessageText", "telegram down")

	err := client.EditMessageWithButtons(context.Background(), 42, 17, "updated", nil, ParseModePlain)

	require.Error(t, err)
	assert.ErrorContains(t, err, "editing telegram message 17 in chat 42")
	assert.ErrorContains(t, err, "telegram down")
}

func TestClient_deletesMessage(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)

	require.NoError(t, client.DeleteMessage(context.Background(), 42, 17))

	request := server.request(t, "deleteMessage")
	assert.Equal(t, "42", request.form.Get("chat_id"))
	assert.Equal(t, "17", request.form.Get("message_id"))
}

func TestClient_deletingMessageReturnsTelegramError(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)
	server.fail("deleteMessage", "telegram down")

	err := client.DeleteMessage(context.Background(), 42, 17)

	require.Error(t, err)
	assert.ErrorContains(t, err, "deleting telegram message 17 in chat 42")
	assert.ErrorContains(t, err, "telegram down")
}

func TestClient_deletesWebhook(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)

	require.NoError(t, client.DeleteWebhook(context.Background()))

	server.request(t, "deleteWebhook")
}

func TestClient_deletingWebhookReturnsTelegramError(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)
	server.fail("deleteWebhook", "telegram down")

	err := client.DeleteWebhook(context.Background())

	require.Error(t, err)
	assert.ErrorContains(t, err, "clearing webhook")
	assert.ErrorContains(t, err, "telegram down")
}

func TestClient_sendsMessageWithKeyboard(t *testing.T) {
	keyboard := [][]string{{"first", "second"}, {"third"}}
	server := newTelegramTestServer(t)
	client := server.client(t)

	err := client.SendMessageWithKeyboard(context.Background(), 42, "menu", keyboard)

	require.NoError(t, err)
	request := server.request(t, "sendMessage")
	assert.Equal(t, "42", request.form.Get("chat_id"))
	assert.Equal(t, "menu", request.form.Get("text"))
	assertReplyKeyboard(t, request.form.Get("reply_markup"), keyboard)
}

func TestClient_sendingMessageWithKeyboardReturnsTelegramError(t *testing.T) {
	server := newTelegramTestServer(t)
	client := server.client(t)
	server.fail("sendMessage", "telegram down")

	err := client.SendMessageWithKeyboard(context.Background(), 42, "menu", [][]string{{"first"}})

	require.Error(t, err)
	assert.ErrorContains(t, err, "sending message with keyboard to chat 42")
	assert.ErrorContains(t, err, "telegram down")
}

func TestInlineRows_preservesRowLabelAndDataShape(t *testing.T) {
	buttons := [][]Button{
		{{Text: "first", Data: "one"}, {Text: "second", Data: "two"}},
		{{Text: "back", Data: "menu"}},
	}

	rows := inlineRows(buttons)

	assert.Equal(t, [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("first", "one"),
			tgbotapi.NewInlineKeyboardButtonData("second", "two"),
		},
		{tgbotapi.NewInlineKeyboardButtonData("back", "menu")},
	}, rows)
}

func assertInlineKeyboard(t *testing.T, raw string, want [][]Button) {
	t.Helper()

	var markup struct {
		InlineKeyboard [][]struct {
			Text         string `json:"text"`
			CallbackData string `json:"callback_data"`
		} `json:"inline_keyboard"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &markup))

	got := make([][]Button, len(markup.InlineKeyboard))
	for i, row := range markup.InlineKeyboard {
		got[i] = make([]Button, len(row))
		for j, button := range row {
			got[i][j] = Button{Text: button.Text, Data: button.CallbackData}
		}
	}
	assert.Equal(t, want, got)
}

func assertReplyKeyboard(t *testing.T, raw string, want [][]string) {
	t.Helper()

	var markup struct {
		Keyboard [][]struct {
			Text string `json:"text"`
		} `json:"keyboard"`
		ResizeKeyboard bool `json:"resize_keyboard"`
		IsPersistent   bool `json:"is_persistent"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &markup))

	got := make([][]string, len(markup.Keyboard))
	for i, row := range markup.Keyboard {
		got[i] = make([]string, len(row))
		for j, button := range row {
			got[i][j] = button.Text
		}
	}
	assert.True(t, markup.ResizeKeyboard)
	assert.True(t, markup.IsPersistent)
	assert.Equal(t, want, got)
}

func TestConvertUpdate(t *testing.T) {
	tests := map[string]struct {
		raw    tgbotapi.Update
		wantOK bool
		want   Update
	}{
		"callback query": {
			raw: tgbotapi.Update{
				CallbackQuery: &tgbotapi.CallbackQuery{
					From: &tgbotapi.User{UserName: "greg"},
					Message: &tgbotapi.Message{
						MessageID: 42,
						Chat:      &tgbotapi.Chat{ID: 1},
					},
					Data: "menu",
				},
			},
			wantOK: true,
			want:   Update{Username: "greg", ChatID: 1, MessageID: 42, Text: "menu", IsCallback: true},
		},
		// An inline-mode callback query carries no Message, so there is nothing
		// to reply into — this used to nil-deref on .Chat.ID.
		"callback query with nil Message": {
			raw: tgbotapi.Update{
				CallbackQuery: &tgbotapi.CallbackQuery{
					From: &tgbotapi.User{UserName: "greg"},
					Data: "menu",
				},
			},
			wantOK: false,
		},
		"message with nil From": {
			raw: tgbotapi.Update{
				Message: &tgbotapi.Message{
					MessageID: 5,
					Chat:      &tgbotapi.Chat{ID: 1},
					Text:      "hi",
				},
			},
			wantOK: false,
		},
		"message with nil Chat": {
			raw: tgbotapi.Update{
				Message: &tgbotapi.Message{
					MessageID: 5,
					From:      &tgbotapi.User{UserName: "greg"},
					Text:      "hi",
				},
			},
			wantOK: false,
		},
		"plain message": {
			raw: tgbotapi.Update{
				Message: &tgbotapi.Message{
					MessageID: 5,
					From:      &tgbotapi.User{UserName: "greg"},
					Chat:      &tgbotapi.Chat{ID: 1},
					Text:      "hi",
				},
			},
			wantOK: true,
			want:   Update{Username: "greg", ChatID: 1, MessageID: 5, Text: "hi"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, ok := convertUpdate(tt.raw)

			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func receiveClientUpdate(t *testing.T, updates <-chan Update) Update {
	t.Helper()

	select {
	case update := <-updates:
		return update
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for client update")
		return Update{}
	}
}

func assertClientUpdatesClosed(t *testing.T, updates <-chan Update) {
	t.Helper()

	select {
	case _, ok := <-updates:
		assert.False(t, ok, "client update channel must close before Listen returns")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for client update channel to close")
	}
}
