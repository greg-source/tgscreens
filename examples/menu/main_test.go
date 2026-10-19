package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/greg-source/tgscreens"
	"github.com/greg-source/tgscreens/telegram"
	"github.com/greg-source/tgscreens/telegram/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// source delivers the given updates in order, then closes the stream.
func source(updates ...telegram.Update) tgscreens.Source {
	return func(_ context.Context, out chan<- telegram.Update) error {
		defer close(out)
		for _, u := range updates {
			out <- u
		}
		return nil
	}
}

func message(text string) telegram.Update {
	return telegram.Update{ChatID: 1, Username: "u", MessageID: 1, Text: text}
}

func press(messageID int, data string) telegram.Update {
	return telegram.Update{ChatID: 1, Username: "u", MessageID: messageID, Text: data, IsCallback: true}
}

func TestMenu_addListAndDeleteAnItem(t *testing.T) {
	sender := mock.NewMockSender(gomock.NewController(t))
	hash := tgscreens.Hash("milk")

	gomock.InOrder(
		sender.EXPECT().SendMessageWithKeyboard(gomock.Any(), int64(1), "Send the text of the new item.", mainKeyboard).Return(nil),
		sender.EXPECT().SendMessageWithKeyboard(gomock.Any(), int64(1), "Added: milk", mainKeyboard).Return(nil),
		sender.EXPECT().SendMessageWithButtons(
			gomock.Any(), int64(1), "Items, page 1 of 1",
			[][]telegram.Button{{{Text: "milk", Data: "item:" + hash}}},
			telegram.ParseModePlain,
		).Return(20, nil),
		sender.EXPECT().EditMessageWithButtons(
			gomock.Any(), int64(1), 20, "milk",
			[][]telegram.Button{
				{{Text: "Delete", Data: "del:" + hash}},
				{{Text: "Back", Data: "list:0"}},
			},
			telegram.ParseModePlain,
		).Return(nil),
		sender.EXPECT().EditMessageWithButtons(
			gomock.Any(), int64(1), 20, "No items yet. Use Add item.", [][]telegram.Button(nil), telegram.ParseModePlain,
		).Return(nil),
	)

	err := newBot(sender).Listen(context.Background(), source(
		message(btnAdd),
		message("milk"),
		message(btnItems),
		press(20, "item:"+hash),
		press(20, "del:"+hash),
	))
	require.NoError(t, err)
}

// Telegram rejects the whole keyboard when one callback exceeds the cap, so the
// worst realistic screen is measured rather than assumed.
func TestMenu_listCallbacksFitTelegramBudget(t *testing.T) {
	sender := mock.NewMockSender(gomock.NewController(t))

	var buttons [][]telegram.Button
	sender.EXPECT().SendMessageWithButtons(gomock.Any(), int64(1), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ int64, _ string, b [][]telegram.Button, _ telegram.ParseMode) (int, error) {
			buttons = b
			return 20, nil
		})
	sender.EXPECT().SendMessageWithKeyboard(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	var updates []telegram.Update
	for i := range 2 * pageSize {
		updates = append(updates, message(btnAdd), message(fmt.Sprintf("a long item name that would overflow the budget if sent raw: %d", i)))
	}
	updates = append(updates, message(btnItems))

	require.NoError(t, newBot(sender).Listen(context.Background(), source(updates...)))

	require.NotEmpty(t, buttons)
	for _, row := range buttons {
		for _, b := range row {
			assert.LessOrEqual(t, len(b.Data), tgscreens.MaxCallbackDataBytes, "callback %q", b.Data)
		}
	}
}
