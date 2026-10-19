package main

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/greg-source/tgscreens"
	"github.com/greg-source/tgscreens/telegram"
	"github.com/greg-source/tgscreens/telegram/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testNow = time.Date(2026, 10, 20, 14, 0, 0, 0, time.UTC)

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

func message(messageID int, text string) telegram.Update {
	return telegram.Update{ChatID: 1, Username: "u", MessageID: messageID, Text: text}
}

func press(messageID int, data string) telegram.Update {
	return telegram.Update{ChatID: 1, Username: "u", MessageID: messageID, Text: data, IsCallback: true}
}

func newTestBot(t *testing.T) (*bot, *mock.MockSender) {
	sender := mock.NewMockSender(gomock.NewController(t))
	b := newBot(sender)
	b.now = func() time.Time { return testNow }
	return b, sender
}

func expectDelete(sender *mock.MockSender, messageID int) *gomock.Call {
	return sender.EXPECT().DeleteMessage(gomock.Any(), int64(1), messageID).Return(nil)
}

func expectSend(sender *mock.MockSender, s tgscreens.Screen, messageID int) *gomock.Call {
	return sender.EXPECT().SendMessageWithButtons(gomock.Any(), int64(1), s.Text, s.Buttons, s.ParseMode).Return(messageID, nil)
}

func expectEdit(sender *mock.MockSender, messageID int, s tgscreens.Screen) *gomock.Call {
	return sender.EXPECT().EditMessageWithButtons(gomock.Any(), int64(1), messageID, s.Text, s.Buttons, s.ParseMode).Return(nil)
}

func TestReminders_createAReminder(t *testing.T) {
	b, sender := newTestBot(t)
	due := testNow.Add(time.Hour)

	gomock.InOrder(
		expectDelete(sender, 1),
		expectSend(sender, writeScreen(""), 10),
		expectDelete(sender, 2),
		expectDelete(sender, 10),
		expectSend(sender, whenScreen("Buy milk"), 11),
		expectEdit(sender, 11, reminderSetScreen(reminder{ID: 1, Text: "Buy milk", Due: due})),
		expectEdit(sender, 11, listScreen([]reminder{{ID: 1, Text: "Buy milk", Due: due}}, false, "", 0)),
	)

	require.NoError(t, b.Listen(context.Background(), source(
		message(1, btnNew),
		message(2, "  Buy milk "),
		press(11, "when:60"),
		press(11, "list:0"),
	)))

	assert.Equal(t, []reminder{{ID: 1, Text: "Buy milk", Due: due}}, b.store.list(1))
}

func TestReminders_writingRepromptsForEmptyText(t *testing.T) {
	b, sender := newTestBot(t)

	gomock.InOrder(
		expectDelete(sender, 1),
		expectSend(sender, writeScreen(""), 10),
		expectDelete(sender, 2),
		expectDelete(sender, 10),
		expectSend(sender, writeScreen(txtTextEmpty), 11),
	)

	require.NoError(t, b.Listen(context.Background(), source(
		message(1, btnNew),
		message(2, "   "),
	)))

	user, _ := b.store.user(1)
	assert.Equal(t, stateWriting, user.State)
}

func TestReminders_keyboardCaptionLeavesWriting(t *testing.T) {
	b, sender := newTestBot(t)

	gomock.InOrder(
		expectDelete(sender, 1),
		expectSend(sender, writeScreen(""), 10),
		expectDelete(sender, 2),
		expectDelete(sender, 10),
		expectSend(sender, settingsScreen(false, ""), 11),
	)

	require.NoError(t, b.Listen(context.Background(), source(
		message(1, btnNew),
		message(2, btnSettings),
	)))

	user, _ := b.store.user(1)
	assert.Equal(t, stateMenu, user.State)
}

func TestReminders_cancelDropsTheDraft(t *testing.T) {
	b, sender := newTestBot(t)

	gomock.InOrder(
		expectDelete(sender, 1),
		expectSend(sender, writeScreen(""), 10),
		expectDelete(sender, 2),
		expectDelete(sender, 10),
		expectSend(sender, whenScreen("Buy milk"), 11),
		expectEdit(sender, 11, listScreen(nil, false, "", 0)),
	)

	require.NoError(t, b.Listen(context.Background(), source(
		message(1, btnNew),
		message(2, "Buy milk"),
		press(11, "cancel"),
	)))

	_, ok := b.store.takeDraft(1)
	assert.False(t, ok)
	assert.Empty(t, b.store.list(1))
}

func TestReminders_openAndDeleteAReminder(t *testing.T) {
	b, sender := newTestBot(t)
	r := b.store.add(1, "Buy milk", testNow.Add(time.Hour))

	gomock.InOrder(
		expectDelete(sender, 1),
		expectSend(sender, listScreen([]reminder{r}, false, "", 0), 20),
		expectEdit(sender, 20, reminderScreen(r, 0)),
		expectEdit(sender, 20, listScreen(nil, false, "🗑 Deleted «Buy milk».", 0)),
		expectEdit(sender, 20, listScreen(nil, false, txtReminderGone, 0)),
	)

	require.NoError(t, b.Listen(context.Background(), source(
		message(1, btnList),
		press(20, "rem:1|0"),
		press(20, "del:1|0"),
		press(20, "rem:1|0"), // a reminder that is gone shows the list again
	)))
}

func TestReminders_settingsToggleAndClear(t *testing.T) {
	b, sender := newTestBot(t)
	b.store.add(1, "Buy milk", testNow.Add(time.Hour))

	gomock.InOrder(
		expectDelete(sender, 1),
		expectSend(sender, settingsScreen(false, ""), 30),
		expectEdit(sender, 30, settingsScreen(true, "")),
		expectEdit(sender, 30, settingsScreen(true, "")),
		expectEdit(sender, 30, clearScreen()),
		expectEdit(sender, 30, settingsScreen(true, txtAllDeleted)),
	)

	require.NoError(t, b.Listen(context.Background(), source(
		message(1, btnSettings),
		press(30, "pause:1"),
		press(30, "pause:1"), // pressed twice, it stays paused
		press(30, "clear"),
		press(30, "clearok"),
	)))

	assert.True(t, b.store.isPaused(1))
	assert.Empty(t, b.store.list(1))
}

func TestReminders_deliverDueSkipsPausedChats(t *testing.T) {
	b, sender := newTestBot(t)
	b.store.add(1, "Buy milk", testNow.Add(-time.Minute))
	b.store.add(1, "Call mom", testNow.Add(time.Hour))
	b.store.add(2, "Water plants", testNow)
	b.store.setPaused(2, true)

	sender.EXPECT().SendMessage(gomock.Any(), int64(1), "⏰ Reminder\n\nBuy milk").Return(nil)
	b.deliverDue(context.Background(), testNow)

	assert.Len(t, b.store.list(1), 1)
	assert.Len(t, b.store.list(2), 1, "paused reminders wait")

	sender.EXPECT().SendMessage(gomock.Any(), int64(2), "⏰ Reminder\n\nWater plants").Return(nil)
	b.store.setPaused(2, false)
	b.deliverDue(context.Background(), testNow)

	assert.Empty(t, b.store.list(2))
}
