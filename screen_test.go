package tgscreens

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/greg-source/tgscreens/telegram"
	"github.com/greg-source/tgscreens/telegram/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testScreen = Screen{
	Text:    "pick one",
	Buttons: [][]Button{{{Text: "a", Data: "pick:a"}}},
}

func newScreenRouter(t *testing.T) (*Router, *mock.MockSender) {
	t.Helper()

	tg := newMockSender(t)
	return newTestRouter(t, tg, &stubStore{}, Config{}), tg
}

func TestShow_editsTheMessageItIsGiven(t *testing.T) {
	r, tg := newScreenRouter(t)
	user := &User{ChatID: 1, MenuMessageID: 42}
	tg.EXPECT().EditMessageWithButtons(gomock.Any(), int64(1), 42, testScreen.Text, testScreen.Buttons, testScreen.ParseMode).Return(nil)

	require.NoError(t, r.Show(context.Background(), user, 42, testScreen))

	assert.Equal(t, 42, user.MenuMessageID)
}

func TestShow_sendsAndTracksWhenThereIsNothingToEdit(t *testing.T) {
	r, tg := newScreenRouter(t)
	user := &User{ChatID: 1}
	tg.EXPECT().SendMessageWithButtons(gomock.Any(), int64(1), testScreen.Text, testScreen.Buttons, testScreen.ParseMode).Return(77, nil)

	require.NoError(t, r.Show(context.Background(), user, 0, testScreen))

	assert.Equal(t, 77, user.MenuMessageID, "the new menu must be tracked so it can be cleaned up")
}

func TestShow_passesHTMLParseModeToTheSender(t *testing.T) {
	r, tg := newScreenRouter(t)
	user := &User{ChatID: 1}
	screen := Screen{Text: "<b>pick one</b>", ParseMode: telegram.ParseModeHTML}
	tg.EXPECT().SendMessageWithButtons(gomock.Any(), int64(1), screen.Text, screen.Buttons, telegram.ParseModeHTML).Return(77, nil)

	require.NoError(t, r.Show(context.Background(), user, 0, screen))
}

// Sending a second menu while one is on screen would strand the first.
func TestShow_deletesTheOldMenuBeforeSendingANewOne(t *testing.T) {
	r, tg := newScreenRouter(t)
	user := &User{ChatID: 1, MenuMessageID: 42}
	gomock.InOrder(
		tg.EXPECT().DeleteMessage(gomock.Any(), int64(1), 42).Return(nil),
		tg.EXPECT().SendMessageWithButtons(gomock.Any(), int64(1), testScreen.Text, testScreen.Buttons, testScreen.ParseMode).Return(77, nil),
	)

	require.NoError(t, r.Show(context.Background(), user, 0, testScreen))

	assert.Equal(t, 77, user.MenuMessageID)
}

func TestShow_sendFailureLeavesNoStaleTracking(t *testing.T) {
	r, tg := newScreenRouter(t)
	user := &User{ChatID: 1}
	tg.EXPECT().SendMessageWithButtons(gomock.Any(), int64(1), testScreen.Text, testScreen.Buttons, testScreen.ParseMode).Return(0, errors.New("telegram down"))

	require.Error(t, r.Show(context.Background(), user, 0, testScreen))
	assert.Zero(t, user.MenuMessageID)
}

func TestKeyboard_deletesOrphanedMenu(t *testing.T) {
	r, tg := newScreenRouter(t)
	user := &User{ChatID: 1, MenuMessageID: 42}
	keyboard := [][]string{{"a"}}
	gomock.InOrder(
		tg.EXPECT().DeleteMessage(gomock.Any(), int64(1), 42).Return(nil),
		tg.EXPECT().SendMessageWithKeyboard(gomock.Any(), int64(1), "menu", keyboard).Return(nil),
	)

	require.NoError(t, r.Keyboard(context.Background(), user, "menu", keyboard))

	assert.Zero(t, user.MenuMessageID)
}

func TestKeyboard_noActiveMenu(t *testing.T) {
	r, tg := newScreenRouter(t)
	user := &User{ChatID: 1}
	keyboard := [][]string{{"a"}}
	tg.EXPECT().SendMessageWithKeyboard(gomock.Any(), int64(1), "menu", keyboard).Return(nil)

	require.NoError(t, r.Keyboard(context.Background(), user, "menu", keyboard))

	assert.Zero(t, user.MenuMessageID)
}

func TestSendKeyboard_keepsTheActiveMenu(t *testing.T) {
	r, tg := newScreenRouter(t)
	user := &User{ChatID: 1, MenuMessageID: 42}
	keyboard := [][]string{{"a"}}
	tg.EXPECT().SendMessageWithKeyboard(gomock.Any(), int64(1), "menu", keyboard).Return(nil)

	require.NoError(t, r.SendKeyboard(context.Background(), user, "menu", keyboard))

	assert.Equal(t, 42, user.MenuMessageID)
}

func TestDiscard_deletesIncomingMessage(t *testing.T) {
	r, tg := newScreenRouter(t)
	tg.EXPECT().DeleteMessage(gomock.Any(), int64(1), 5).Return(nil)

	r.Discard(context.Background(), 1, 5)
}

func TestDiscard_reportsFailedDeletion(t *testing.T) {
	tg := newMockSender(t)
	boom := errors.New("message to delete not found")
	tg.EXPECT().DeleteMessage(gomock.Any(), int64(1), 42).Return(boom)

	var reportedID int
	var reportedErr error
	r := newTestRouter(t, tg, &stubStore{}, Config{
		OnCleanupError: func(_ context.Context, _ int64, messageID int, err error) {
			reportedID, reportedErr = messageID, err
		},
	})

	r.Discard(context.Background(), 1, 42)

	assert.Equal(t, 42, reportedID)
	require.ErrorIs(t, reportedErr, boom)
}

// OnCleanupError is optional: a nil hook must not panic a failed cleanup.
func TestDiscard_toleratesNoOnCleanupErrorHook(t *testing.T) {
	tg := newMockSender(t)
	tg.EXPECT().DeleteMessage(gomock.Any(), int64(1), 42).Return(errors.New("gone"))
	r := newTestRouter(t, tg, &stubStore{}, Config{})

	assert.NotPanics(t, func() {
		r.Discard(context.Background(), 1, 42)
	})
}
