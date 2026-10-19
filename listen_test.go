package tgscreens

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/greg-source/tgscreens/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Listen is driven entirely by the Source it is given, with no dependency on
// tgbotapi — that seam keeps the router testable without a network.
func TestListen_dispatchesUpdatesInOrder(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", State: 7}}

	var got []string
	r := newTestRouter(t, newMockSender(t), store, Config{
		MessageHandlers: map[State]MessageHandlerFunc{
			7: func(_ context.Context, mc MessageContext, _ *User) error {
				got = append(got, "message:"+mc.Text)
				return nil
			},
		},
		CallbackHandlers: map[Action]CallbackHandlerFunc{
			"menu": func(_ context.Context, cc CallbackContext, _ *User) error {
				got = append(got, "callback:"+string(cc.Action))
				return nil
			},
		},
	})

	src := func(_ context.Context, out chan<- telegram.Update) error {
		defer close(out)
		out <- telegram.Update{ChatID: 1, Username: "u", Text: "hi"}
		out <- telegram.Update{ChatID: 1, Username: "u", Text: "menu", IsCallback: true}
		return nil
	}

	err := r.Listen(context.Background(), src)

	require.NoError(t, err)
	assert.Equal(t, []string{"message:hi", "callback:menu"}, got)
}

func TestListen_returnsSourceError(t *testing.T) {
	r := newTestRouter(t, newMockSender(t), &stubStore{}, Config{})

	boom := errors.New("boom")
	src := func(_ context.Context, out chan<- telegram.Update) error {
		close(out)
		return boom
	}

	err := r.Listen(context.Background(), src)

	assert.ErrorIs(t, err, boom)
}

func TestListen_returnsCtxErrWhenSourceStopsClean(t *testing.T) {
	r := newTestRouter(t, newMockSender(t), &stubStore{}, Config{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	src := func(_ context.Context, out chan<- telegram.Update) error {
		close(out)
		return nil
	}

	err := r.Listen(ctx, src)

	assert.ErrorIs(t, err, context.Canceled)
}

// The out=nil branch is what stops Listen from busy-looping on a closed
// channel while it waits for the source to report why it stopped.
func TestListen_drainsUpdatesQueuedBeforeClose(t *testing.T) {
	store := &stubStore{user: User{ChatID: 1, Username: "u", State: 7}}
	r := newTestRouter(t, newMockSender(t), store, Config{
		MessageHandlers: map[State]MessageHandlerFunc{7: noopMessage},
	})

	boom := errors.New("boom")
	src := func(_ context.Context, out chan<- telegram.Update) error {
		out <- telegram.Update{ChatID: 1, Username: "u", Text: "one"}
		out <- telegram.Update{ChatID: 1, Username: "u", Text: "two"}
		close(out)
		time.Sleep(20 * time.Millisecond)
		return boom
	}

	err := r.Listen(context.Background(), src)

	assert.ErrorIs(t, err, boom)
	assert.Equal(t, 2, store.loadCall, "both updates dispatched exactly once, none spuriously replayed after close")
}
