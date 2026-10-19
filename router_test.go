package tgscreens

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/greg-source/tgscreens/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoute_dispatchesByState(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", State: 7}}

	var got string
	r := newTestRouter(t, newMockSender(t), store, Config{
		MessageHandlers: map[State]MessageHandlerFunc{
			7: func(_ context.Context, mc MessageContext, _ *User) error { got = "seven:" + mc.Text; return nil },
			8: func(context.Context, MessageContext, *User) error { got = "eight"; return nil },
		},
		OnUnknownMessage: func(context.Context, MessageContext, *User) error { got = "unknown"; return nil },
	})

	r.route(context.Background(), &telegram.Update{ChatID: 1, Username: "u", Text: "hi"})

	assert.Equal(t, "seven:hi", got)
	assert.Empty(t, store.saved, "nothing changed, so nothing should be saved")
}

func TestRoute_unregisteredStateFallsBack(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", State: 99}}

	var got string
	r := newTestRouter(t, newMockSender(t), store, Config{
		MessageHandlers:  map[State]MessageHandlerFunc{7: noopMessage},
		OnUnknownMessage: func(context.Context, MessageContext, *User) error { got = "unknown"; return nil },
	})

	r.route(context.Background(), &telegram.Update{ChatID: 1, Username: "u", Text: "hi"})

	assert.Equal(t, "unknown", got)
}

// A button left in chat by an older deploy must reach OnUnknownCallback, not
// whichever handler happens to sit at the zero value of the action type.
func TestRoute_unregisteredActionFallsBack(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u"}}

	var got string
	r := newTestRouter(t, newMockSender(t), store, Config{
		CallbackHandlers: map[Action]CallbackHandlerFunc{
			"menu": func(context.Context, CallbackContext, *User) error { got = "menu"; return nil },
		},
		OnUnknownCallback: func(_ context.Context, cc CallbackContext, _ *User) error {
			got = "unknown:" + string(cc.Action)
			return nil
		},
	})

	r.route(context.Background(), &telegram.Update{ChatID: 1, Username: "u", Text: "3", IsCallback: true})

	assert.Equal(t, "unknown:3", got)
}

func TestRoute_passesCallbackArgs(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u"}}

	var got CallbackContext
	r := newTestRouter(t, newMockSender(t), store, Config{
		CallbackHandlers: map[Action]CallbackHandlerFunc{
			"pick": func(_ context.Context, cc CallbackContext, _ *User) error { got = cc; return nil },
		},
	})

	r.route(context.Background(), &telegram.Update{
		ChatID: 1, Username: "u", Text: "pick:a3f9c012|2", IsCallback: true,
	})

	assert.Equal(t, Action("pick"), got.Action)
	assert.Equal(t, []string{"a3f9c012", "2"}, got.Args)
}

func TestRoute_savesOnceWhenHandlerChangesUser(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", State: 1}}

	r := newTestRouter(t, newMockSender(t), store, Config{
		MessageHandlers: map[State]MessageHandlerFunc{
			1: func(_ context.Context, _ MessageContext, user *User) error {
				user.State = 2
				return nil
			},
		},
	})

	r.route(context.Background(), &telegram.Update{ChatID: 1, Username: "u", Text: "hi"})

	require.Len(t, store.saved, 1)
	assert.Equal(t, State(2), store.saved[0].State)
}

// A handler may show a menu and then fail. Dropping the save would leave that
// message tracked nowhere and impossible to clean up.
func TestRoute_savesEvenWhenHandlerFails(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", State: 1}}

	var reported error
	r := newTestRouter(t, newMockSender(t), store, Config{
		MessageHandlers: map[State]MessageHandlerFunc{
			1: func(_ context.Context, _ MessageContext, user *User) error {
				user.MenuMessageID = 555
				return errors.New("boom")
			},
		},
		OnError: func(_ context.Context, _ int64, _ *User, err error) { reported = err },
	})

	r.route(context.Background(), &telegram.Update{ChatID: 1, Username: "u", Text: "hi"})

	require.Len(t, store.saved, 1)
	assert.Equal(t, 555, store.saved[0].MenuMessageID)
	require.Error(t, reported)
	assert.Contains(t, reported.Error(), "boom")
}

func TestRoute_persistsNoticeShownByOnError(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", State: 7}}
	boom := errors.New("boom")
	notice := Screen{Text: "retry"}
	tg := newMockSender(t)
	tg.EXPECT().SendMessageWithButtons(gomock.Any(), int64(1), notice.Text, notice.Buttons, notice.ParseMode).Return(555, nil)

	var r *Router
	r = newTestRouter(t, tg, store, Config{
		MessageHandlers: map[State]MessageHandlerFunc{
			7: func(_ context.Context, _ MessageContext, user *User) error {
				user.State = 8
				return boom
			},
		},
		OnError: func(ctx context.Context, _ int64, user *User, _ error) {
			store.order = append(store.order, "error")
			require.NoError(t, r.Show(ctx, user, 0, notice))
		},
	})

	r.route(context.Background(), &telegram.Update{ChatID: 1, Username: "u", Text: "hi"})

	require.Len(t, store.saved, 2)
	assert.Equal(t, User{ChatID: 1, Username: "u", State: 8}, store.saved[0])
	assert.Equal(t, User{ChatID: 1, Username: "u", State: 8, MenuMessageID: 555}, store.saved[1])
	assert.Equal(t, []string{"save", "error", "save", "action"}, store.order)
	require.Len(t, store.actions, 1)
	assert.ErrorIs(t, store.actions[0].err, boom)
}

// The message a button sits on becomes the tracked menu, and that adoption is
// itself a change: without persisting it the message is orphaned.
func TestRoute_persistsAdoptedMenuMessage(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u"}}

	r := newTestRouter(t, newMockSender(t), store, Config{
		CallbackHandlers: map[Action]CallbackHandlerFunc{"menu": noopCallback},
	})

	r.route(context.Background(), &telegram.Update{
		ChatID: 1, Username: "u", MessageID: 42, Text: "menu", IsCallback: true,
	})

	require.Len(t, store.saved, 1)
	assert.Equal(t, 42, store.saved[0].MenuMessageID)
}

// An old inline keyboard can still be pressed after a newer menu is on screen.
// It must not replace the newer menu's tracking or run its old action.
func TestRoute_callbackFromDifferentMenuUsesFallbackWithoutReplacingTrackedMenu(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", MenuMessageID: 100}}

	called := false
	fallbackCalled := false
	fallbackMenuID := 0
	r := newTestRouter(t, newMockSender(t), store, Config{
		CallbackHandlers: map[Action]CallbackHandlerFunc{
			"known": func(context.Context, CallbackContext, *User) error {
				called = true
				return nil
			},
		},
		OnUnknownCallback: func(_ context.Context, _ CallbackContext, user *User) error {
			fallbackCalled = true
			fallbackMenuID = user.MenuMessageID
			return nil
		},
	})

	r.route(context.Background(), &telegram.Update{
		ChatID: 1, Username: "u", MessageID: 42, Text: "known", IsCallback: true,
	})

	assert.False(t, called)
	assert.True(t, fallbackCalled)
	assert.Equal(t, 100, fallbackMenuID)
	assert.Empty(t, store.saved, "the active menu was not changed")
}

// A callback carrying no message must not blank the tracked menu: forgetting
// the ID without deleting the message orphans it permanently.
func TestRoute_callbackWithoutAMessageKeepsTheTrackedMenu(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", MenuMessageID: 42}}

	var seen int
	r := newTestRouter(t, newMockSender(t), store, Config{
		CallbackHandlers: map[Action]CallbackHandlerFunc{
			"menu": func(_ context.Context, _ CallbackContext, user *User) error {
				seen = user.MenuMessageID
				return nil
			},
		},
	})

	r.route(context.Background(), &telegram.Update{
		ChatID: 1, Username: "u", MessageID: 0, Text: "menu", IsCallback: true,
	})

	assert.Equal(t, 42, seen)
	assert.Empty(t, store.saved, "nothing changed, so nothing should be saved")
}

func TestRoute_saveFailureIsReported(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", State: 1}, saveErr: errors.New("dynamo down")}

	var reported error
	r := newTestRouter(t, newMockSender(t), store, Config{
		MessageHandlers: map[State]MessageHandlerFunc{
			1: func(_ context.Context, _ MessageContext, user *User) error { user.State = 2; return nil },
		},
		OnError: func(_ context.Context, _ int64, _ *User, err error) { reported = err },
	})

	r.route(context.Background(), &telegram.Update{ChatID: 1, Username: "u", Text: "hi"})

	require.Error(t, reported)
	assert.Contains(t, reported.Error(), "dynamo down")
}

func TestRoute_loadFailureSkipsHandlers(t *testing.T) {
	store := &stubStore{loadErr: errors.New("no user")}

	called := false
	var reported error
	var reportedUser *User
	r := newTestRouter(t, newMockSender(t), store, Config{
		OnUnknownMessage: func(context.Context, MessageContext, *User) error { called = true; return nil },
		OnError: func(_ context.Context, _ int64, user *User, err error) {
			reportedUser, reported = user, err
		},
	})

	r.route(context.Background(), &telegram.Update{ChatID: 1, Username: "u", Text: "hi"})

	assert.False(t, called)
	assert.Empty(t, store.saved)
	assert.Nil(t, reportedUser)
	require.Error(t, reported)
	assert.Contains(t, reported.Error(), "no user")
}

func TestNew_panicsOnMissingDependency(t *testing.T) {
	full := Config{
		LoadUser:          func(context.Context, string, int64) (User, error) { return User{}, nil },
		SaveUser:          func(context.Context, User) error { return nil },
		OnUnknownMessage:  noopMessage,
		OnUnknownCallback: noopCallback,
		OnError:           noopError,
		OnAction:          noopAction,
	}

	tests := map[string]func(*Config){
		"LoadUser":          func(c *Config) { c.LoadUser = nil },
		"SaveUser":          func(c *Config) { c.SaveUser = nil },
		"OnUnknownMessage":  func(c *Config) { c.OnUnknownMessage = nil },
		"OnUnknownCallback": func(c *Config) { c.OnUnknownCallback = nil },
		"OnError":           func(c *Config) { c.OnError = nil },
		"OnAction":          func(c *Config) { c.OnAction = nil },
	}

	for name, clear := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := full
			clear(&cfg)
			assert.Panics(t, func() { New(newMockSender(t), cfg) })
		})
	}
}

func TestNew_panicsOnNilSender(t *testing.T) {
	cfg := Config{
		LoadUser:          func(context.Context, string, int64) (User, error) { return User{}, nil },
		SaveUser:          func(context.Context, User) error { return nil },
		OnUnknownMessage:  noopMessage,
		OnUnknownCallback: noopCallback,
		OnError:           noopError,
		OnAction:          noopAction,
	}

	assert.Panics(t, func() { New(nil, cfg) })
}

// FR-16: every update reaches OnAction exactly once, whatever it was and however
// it went. The package keeps no record itself, so a missed call is a lost record.
func TestRoute_reportsMessageAction(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", State: 7}}

	r := newTestRouter(t, newMockSender(t), store, Config{
		MessageHandlers: map[State]MessageHandlerFunc{7: noopMessage},
	})

	r.route(context.Background(), &telegram.Update{ChatID: 1, Username: "u", Text: "hi"})

	require.Len(t, store.actions, 1)
	assert.Equal(t, "hi", store.actions[0].action)
	assert.False(t, store.actions[0].isCallback)
	assert.Equal(t, State(7), store.actions[0].user.State, "the state the update was dispatched in")
	assert.NoError(t, store.actions[0].err)
}

func TestRoute_reportsCallbackActionAsRawData(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", State: 7}}

	r := newTestRouter(t, newMockSender(t), store, Config{
		CallbackHandlers: map[Action]CallbackHandlerFunc{"pick": noopCallback},
	})

	r.route(context.Background(), &telegram.Update{ChatID: 1, Username: "u", MessageID: 9, Text: "pick:a3f9c012|1", IsCallback: true})

	require.Len(t, store.actions, 1)
	assert.Equal(t, "pick:a3f9c012|1", store.actions[0].action, "the wire format, not a decoded form")
	assert.True(t, store.actions[0].isCallback)
}

// A failed update is the one most worth having a record of.
func TestRoute_reportsHandlerFailureAndStillCallsOnError(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", State: 7}}
	boom := errors.New("boom")

	var reported error
	r := newTestRouter(t, newMockSender(t), store, Config{
		MessageHandlers: map[State]MessageHandlerFunc{
			7: func(context.Context, MessageContext, *User) error { return boom },
		},
		OnError: func(_ context.Context, _ int64, _ *User, err error) { reported = err },
	})

	r.route(context.Background(), &telegram.Update{ChatID: 1, Username: "u", Text: "hi"})

	require.Len(t, store.actions, 1)
	assert.ErrorIs(t, store.actions[0].err, boom)
	assert.ErrorIs(t, reported, boom, "OnError still fires; the two hooks are independent")
}

func TestRoute_reportsWhenUserCannotBeLoaded(t *testing.T) {
	store := &stubStore{loadErr: errors.New("dynamo down")}

	r := newTestRouter(t, newMockSender(t), store, Config{OnError: noopError})

	r.route(context.Background(), &telegram.Update{ChatID: 42, Username: "u", Text: "hi"})

	require.Len(t, store.actions, 1, "an action that failed before its user loaded is still an action")
	assert.Equal(t, int64(42), store.actions[0].user.ChatID)
	assert.Equal(t, "u", store.actions[0].user.Username, "what Telegram sent, there being no record")
	assert.Error(t, store.actions[0].err)
}

// The record carries the error, so it has to be taken after the save could add
// one — and the saved menu ID has to be durable before it is reported.
func TestRoute_reportsAfterSaving(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", State: 7}}

	r := newTestRouter(t, newMockSender(t), store, Config{
		MessageHandlers: map[State]MessageHandlerFunc{
			7: func(_ context.Context, _ MessageContext, u *User) error { u.State = 8; return nil },
		},
	})

	r.route(context.Background(), &telegram.Update{ChatID: 1, Username: "u", Text: "hi"})

	assert.Equal(t, []string{"save", "action"}, store.order)
}

func TestRoute_reportsSaveFailureInTheAction(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", State: 7}, saveErr: errors.New("save failed")}

	r := newTestRouter(t, newMockSender(t), store, Config{
		MessageHandlers: map[State]MessageHandlerFunc{
			7: func(_ context.Context, _ MessageContext, u *User) error { u.State = 8; return nil },
		},
		OnError: noopError,
	})

	r.route(context.Background(), &telegram.Update{ChatID: 1, Username: "u", Text: "hi"})

	require.Len(t, store.actions, 1)
	assert.ErrorContains(t, store.actions[0].err, "save failed", "a failed save is a failure the user experiences")
}
