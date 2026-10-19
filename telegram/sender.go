package telegram

import "context"

//go:generate mockgen -source=sender.go -destination=mock/mock_sender.go -package=mock

// ParseMode controls Telegram's formatting of message text.
type ParseMode string

const (
	ParseModePlain ParseMode = ""
	ParseModeHTML  ParseMode = "HTML"
)

// Sender sends and manages messages in a Telegram chat.
type Sender interface {
	SendMessage(ctx context.Context, chatID int64, message string) error
	SendMessageWithButtons(ctx context.Context, chatID int64, message string, buttons [][]Button, parseMode ParseMode) (int, error)
	EditMessageWithButtons(ctx context.Context, chatID int64, messageID int, message string, buttons [][]Button, parseMode ParseMode) error
	DeleteMessage(ctx context.Context, chatID int64, messageID int) error
	SendMessageWithKeyboard(ctx context.Context, chatID int64, message string, keyboard [][]string) error
}

// Button is an inline button below a bot message; Data returns in a callback.
type Button struct {
	Text string
	Data string
}

var _ Sender = (*Client)(nil)
