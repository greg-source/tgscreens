package telegram

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Client is a thin wrapper around the Telegram Bot API: receiving updates,
// and sending, editing, and deleting messages. It knows nothing about
// dispatch, conversation state, or any particular bot's domain.
type Client struct {
	bot *tgbotapi.BotAPI
}

// persistentKeyboard adds the Bot API field missing from tgbotapi v5.5.1.
type persistentKeyboard struct {
	tgbotapi.ReplyKeyboardMarkup
	IsPersistent bool `json:"is_persistent"`
}

// Update is one incoming interaction, translated out of the Telegram API's
// own types before it reaches a router.
type Update struct {
	Username   string
	ChatID     int64
	MessageID  int
	Text       string
	IsCallback bool
}

// convertUpdate translates a raw Telegram update into the package's own type.
// ok is false for anything with no chat to reply into — an inline-mode
// callback query carries no Message, and a channel post with no sender is not
// something a conversation state machine can act on.
func convertUpdate(raw tgbotapi.Update) (u Update, ok bool) {
	if raw.CallbackQuery != nil {
		if raw.CallbackQuery.Message == nil {
			return Update{}, false
		}
		return Update{
			Username:   raw.CallbackQuery.From.UserName,
			ChatID:     raw.CallbackQuery.Message.Chat.ID,
			MessageID:  raw.CallbackQuery.Message.MessageID,
			Text:       raw.CallbackQuery.Data,
			IsCallback: true,
		}, true
	}

	if raw.Message == nil || raw.Message.From == nil || raw.Message.Chat == nil {
		return Update{}, false
	}
	return Update{
		Username:  raw.Message.From.UserName,
		ChatID:    raw.Message.Chat.ID,
		MessageID: raw.Message.MessageID,
		Text:      raw.Message.Text,
	}, true
}

// New creates a Client, validating token against the Telegram API.
func New(token string) (*Client, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("creating telegram client: %w", err)
	}
	return &Client{bot: bot}, nil
}

// DeleteWebhook clears any registered webhook, so a later Poll won't 409.
func (c *Client) DeleteWebhook(_ context.Context) error {
	if _, err := c.bot.Request(tgbotapi.DeleteWebhookConfig{}); err != nil {
		return fmt.Errorf("clearing webhook: %w", err)
	}
	return nil
}

// SendMessage sends a text message to the specified chat.
func (c *Client) SendMessage(_ context.Context, chatID int64, message string) error {
	msg := tgbotapi.NewMessage(chatID, message)
	if _, err := c.bot.Send(msg); err != nil {
		return fmt.Errorf("sending telegram message to chat %d: %w", chatID, err)
	}

	return nil
}

// SendMessageWithButtons sends a text message with inline keyboard buttons to
// the specified chat, parsing it according to parseMode. It returns the message
// ID of the sent message.
func (c *Client) SendMessageWithButtons(_ context.Context, chatID int64, message string, buttons [][]Button, parseMode ParseMode) (int, error) {
	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = string(parseMode)

	if len(buttons) > 0 {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(inlineRows(buttons)...)
	}

	resp, err := c.bot.Send(msg)
	if err != nil {
		return 0, fmt.Errorf("sending telegram message with buttons to chat %d: %w", chatID, err)
	}

	return resp.MessageID, nil
}

// EditMessageWithButtons edits an existing message's text and inline keyboard buttons.
func (c *Client) EditMessageWithButtons(_ context.Context, chatID int64, messageID int, message string, buttons [][]Button, parseMode ParseMode) error {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, message)
	edit.ParseMode = string(parseMode)

	// Built directly, not via tgbotapi.NewInlineKeyboardMarkup: that helper's
	// append collapses to nil for zero rows, and a nil inline_keyboard is
	// rejected by the Bot API where an empty Array is required.
	markup := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: inlineRows(buttons)}
	edit.ReplyMarkup = &markup

	if _, err := c.bot.Send(edit); err != nil {
		if isMessageNotModified(err) {
			return nil
		}
		return fmt.Errorf("editing telegram message %d in chat %d: %w", messageID, chatID, err)
	}

	return nil
}

// isMessageNotModified reports whether the Telegram API error indicates that
// the message content is identical to the current one.
func isMessageNotModified(err error) bool {
	return strings.Contains(err.Error(), "message is not modified")
}

// DeleteMessage deletes a message from the specified chat.
func (c *Client) DeleteMessage(_ context.Context, chatID int64, messageID int) error {
	del := tgbotapi.NewDeleteMessage(chatID, messageID)
	if _, err := c.bot.Request(del); err != nil {
		return fmt.Errorf("deleting telegram message %d in chat %d: %w", messageID, chatID, err)
	}
	return nil
}

// SendMessageWithKeyboard sends labels in a reply keyboard; each inner slice is one row.
func (c *Client) SendMessageWithKeyboard(_ context.Context, chatID int64, message string, keyboard [][]string) error {
	keyboardRows := make([][]tgbotapi.KeyboardButton, 0, len(keyboard))
	for _, labels := range keyboard {
		keyboardRow := make([]tgbotapi.KeyboardButton, 0, len(labels))
		for _, label := range labels {
			keyboardRow = append(keyboardRow, tgbotapi.NewKeyboardButton(label))
		}
		keyboardRows = append(keyboardRows, keyboardRow)
	}
	markup := persistentKeyboard{
		ReplyKeyboardMarkup: tgbotapi.NewReplyKeyboard(keyboardRows...),
		IsPersistent:        true,
	}
	msg := tgbotapi.NewMessage(chatID, message)
	msg.ReplyMarkup = markup
	if _, err := c.bot.Send(msg); err != nil {
		return fmt.Errorf("sending message with keyboard to chat %d: %w", chatID, err)
	}
	return nil
}

// ackCallback clears the button spinner before the update is converted.
func (c *Client) ackCallback(raw tgbotapi.Update) {
	if raw.CallbackQuery == nil {
		return
	}
	c.bot.Send(tgbotapi.NewCallback(raw.CallbackQuery.ID, ""))
}

// inlineRows always returns a non-nil slice, even for zero buttons — see
// EditMessageWithButtons for why that matters.
func inlineRows(buttons [][]Button) [][]tgbotapi.InlineKeyboardButton {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(buttons))
	for _, row := range buttons {
		keyboardRow := make([]tgbotapi.InlineKeyboardButton, 0, len(row))
		for _, btn := range row {
			keyboardRow = append(keyboardRow, tgbotapi.NewInlineKeyboardButtonData(btn.Text, btn.Data))
		}
		rows = append(rows, keyboardRow)
	}
	return rows
}
