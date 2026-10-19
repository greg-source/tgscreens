package tgscreens

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/greg-source/tgscreens/telegram"
	"github.com/greg-source/tgscreens/telegram/mock"
)

type recordedAction struct {
	user       User
	action     string
	isCallback bool
	err        error
}

// stubStore stands in for a bot's storage, recording every save and action so
// tests can assert on what the router decided to persist and report.
type stubStore struct {
	user    User
	saved   []User
	actions []recordedAction
	// order interleaves saves and reports, so a test can assert the hook fires
	// after the user is persisted rather than merely that both happened.
	order    []string
	loadErr  error
	saveErr  error
	loadCall int
}

func (s *stubStore) load(_ context.Context, _ string, _ int64) (User, error) {
	s.loadCall++
	if s.loadErr != nil {
		return User{}, s.loadErr
	}
	return s.user, nil
}

func (s *stubStore) save(_ context.Context, u User) error {
	s.saved = append(s.saved, u)
	s.order = append(s.order, "save")
	return s.saveErr
}

func (s *stubStore) record(_ context.Context, u User, action string, isCallback bool, err error) {
	s.actions = append(s.actions, recordedAction{user: u, action: action, isCallback: isCallback, err: err})
	s.order = append(s.order, "action")
}

func noopMessage(context.Context, MessageContext, *User) error   { return nil }
func noopCallback(context.Context, CallbackContext, *User) error { return nil }
func noopError(context.Context, int64, *User, error)             {}
func noopAction(context.Context, User, string, bool, error)      {}

func newMockSender(t *testing.T) *mock.MockSender {
	t.Helper()

	return mock.NewMockSender(gomock.NewController(t))
}

// newTestRouter fills the fields New insists on, so each test only states the
// ones it actually cares about.
func newTestRouter(t *testing.T, tg telegram.Sender, store *stubStore, cfg Config) *Router {
	t.Helper()

	cfg.LoadUser = store.load
	cfg.SaveUser = store.save
	if cfg.OnUnknownMessage == nil {
		cfg.OnUnknownMessage = noopMessage
	}
	if cfg.OnUnknownCallback == nil {
		cfg.OnUnknownCallback = noopCallback
	}
	if cfg.OnError == nil {
		cfg.OnError = noopError
	}
	if cfg.OnAction == nil {
		cfg.OnAction = store.record
	}

	return New(tg, cfg)
}
