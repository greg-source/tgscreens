package main

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/greg-source/tgscreens/telegram"
	"github.com/greg-source/tgscreens/telegram/mock"
	"github.com/stretchr/testify/require"
)

func TestEcho_messageShowsButtonAndPingEditsInPlace(t *testing.T) {
	sender := mock.NewMockSender(gomock.NewController(t))

	gomock.InOrder(
		sender.EXPECT().SendMessageWithButtons(
			gomock.Any(), int64(1), "💬 You said: hello",
			[][]telegram.Button{{{Text: "🏓 Ping", Data: "ping:1"}}},
			telegram.ParseModePlain,
		).Return(10, nil),
		sender.EXPECT().EditMessageWithButtons(
			gomock.Any(), int64(1), 10, "🏓 Pong #1",
			[][]telegram.Button{{{Text: "🏓 Ping", Data: "ping:2"}}},
			telegram.ParseModePlain,
		).Return(nil),
	)

	source := func(_ context.Context, out chan<- telegram.Update) error {
		defer close(out)
		out <- telegram.Update{ChatID: 1, Username: "u", MessageID: 5, Text: "hello"}
		out <- telegram.Update{ChatID: 1, Username: "u", MessageID: 10, Text: "ping:1", IsCallback: true}
		return nil
	}

	require.NoError(t, newBot(sender).Listen(context.Background(), source))
}
