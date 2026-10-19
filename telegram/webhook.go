package telegram

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gofiber/fiber/v3"
)

var allowedWebhookPorts = []string{"443", "80", "88", "8443"}

// Webhook is a tgscreens.Source that receives updates Telegram pushes to an
// HTTPS endpoint, in place of Poll's long polling.
type Webhook struct {
	client    *Client
	publicURL string
	addr      string
	path      string
	certFile  string
	keyFile   string
}

// NewWebhook validates publicURL. The bind address is derived from its port
// alone — the public host is never locally bindable behind a NAT'd EIP.
func NewWebhook(c *Client, publicURL, certFile, keyFile string) (*Webhook, error) {
	u, err := url.Parse(publicURL)
	if err != nil {
		return nil, fmt.Errorf("webhook: parsing URL: %w", err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("webhook: URL scheme must be https, got %q", u.Scheme)
	}
	if strings.Trim(u.Path, "/") == "" {
		return nil, fmt.Errorf("webhook: URL must have a path")
	}
	port := u.Port()
	if port == "" {
		port = "443" // u.Port() only returns an explicit port; https defaults to 443
	}
	if !slices.Contains(allowedWebhookPorts, port) {
		return nil, fmt.Errorf("webhook: URL port must be one of %v, got %q", allowedWebhookPorts, port)
	}

	return &Webhook{
		client:    c,
		publicURL: publicURL,
		addr:      ":" + port,
		path:      u.Path,
		certFile:  certFile,
		keyFile:   keyFile,
	}, nil
}

// Listen is a tgscreens.Source that serves the registered webhook until ctx cancels.
func (w *Webhook) Listen(ctx context.Context, out chan<- Update) error {
	defer close(out)

	app := fiber.New()
	app.Post(w.path, w.handle(ctx, out))

	if err := app.Listen(w.addr, fiber.ListenConfig{
		CertFile:              w.certFile,
		CertKeyFile:           w.keyFile,
		GracefulContext:       ctx,
		DisableStartupMessage: true,
	}); err != nil {
		return fmt.Errorf("serving webhook: %w", err)
	}

	if err := w.client.DeleteWebhook(ctx); err != nil {
		return fmt.Errorf("clearing webhook after shutdown: %w", err)
	}
	return ctx.Err()
}

// Register calls setWebhook so Telegram starts posting to w.path; call it before Listen.
func (w *Webhook) Register() error {
	wc, err := tgbotapi.NewWebhookWithCert(w.publicURL, tgbotapi.FilePath(w.certFile))
	if err != nil {
		return fmt.Errorf("webhook: building config: %w", err)
	}
	wc.MaxConnections = 1 // default is 40; the router needs updates in order

	// TODO: extend for edited_message, channel_post, inline_query, poll, chat_member, etc. if ever needed.
	wc.AllowedUpdates = []string{"message", "callback_query"}

	if _, err := w.client.bot.Request(wc); err != nil {
		return fmt.Errorf("webhook: registering: %w", err)
	}
	return nil
}

// handle is the Fiber route for w.path.
func (w *Webhook) handle(ctx context.Context, out chan<- Update) fiber.Handler {
	return func(c fiber.Ctx) error {
		var raw tgbotapi.Update
		if err := c.Bind().Body(&raw); err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		w.client.ackCallback(raw)

		u, ok := convertUpdate(raw)
		if !ok {
			return c.SendStatus(fiber.StatusOK)
		}

		select {
		case out <- u:
			return c.SendStatus(fiber.StatusOK)
		case <-ctx.Done():
			return c.SendStatus(fiber.StatusServiceUnavailable)
		}
	}
}
