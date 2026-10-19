package telegram

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Poller is a tgscreens.Source that receives updates via long polling.
type Poller struct {
	client *Client
}

// NewPoller builds a Poller that reuses c's already-authenticated bot.
func NewPoller(c *Client) *Poller {
	return &Poller{client: c}
}

// Poll receives updates via long polling and forwards them to out, closing it before returning.
func (p *Poller) Poll(ctx context.Context, out chan<- Update) error {
	defer close(out)

	updates := p.client.bot.GetUpdatesChan(tgbotapi.UpdateConfig{Timeout: 60})

	for {
		select {
		case raw := <-updates:
			p.client.ackCallback(raw)

			u, ok := convertUpdate(raw)
			if !ok {
				continue
			}
			select {
			case out <- u:
			case <-ctx.Done():
				p.client.bot.StopReceivingUpdates()
				return ctx.Err()
			}
		case <-ctx.Done():
			p.client.bot.StopReceivingUpdates()
			return ctx.Err()
		}
	}
}
