package tgscreens

import (
	"context"
	"errors"
	"fmt"

	"github.com/greg-source/tgscreens/telegram"
)

// Listen receives updates from src and dispatches them to the configured
// handlers.
func (r *Router) Listen(ctx context.Context, src Source) error {
	out := make(chan telegram.Update)
	errCh := make(chan error, 1)
	go func() { errCh <- src(ctx, out) }()

	for {
		select {
		case u, ok := <-out:
			if !ok {
				// The source is winding down; keep draining until it reports why.
				out = nil
				continue
			}
			r.route(ctx, &u)
		case err := <-errCh:
			if err != nil {
				return err
			}
			return ctx.Err()
		}
	}
}

// route handles one update.
func (r *Router) route(ctx context.Context, u *telegram.Update) {
	loaded := User{ChatID: u.ChatID, Username: u.Username}
	var handleErr error
	defer func() { r.cfg.OnAction(ctx, loaded, u.Text, u.IsCallback, handleErr) }()

	user, err := r.cfg.LoadUser(ctx, u.Username, u.ChatID)
	if err != nil {
		handleErr = fmt.Errorf("resolving user %s: %w", u.Username, err)
		r.cfg.OnError(ctx, u.ChatID, nil, handleErr)
		return
	}
	loaded = user

	if u.IsCallback {
		handleErr = r.routeCallback(ctx, u, &user)
	} else {
		handleErr = r.routeMessage(ctx, u, &user)
	}

	handleErr = errors.Join(handleErr, r.persist(ctx, user, loaded))
	if handleErr == nil {
		return
	}

	shown := user
	r.cfg.OnError(ctx, u.ChatID, &user, handleErr)
	handleErr = errors.Join(handleErr, r.persist(ctx, user, shown))
}

// persist saves user only when a handler changed it since before.
func (r *Router) persist(ctx context.Context, user, before User) error {
	if user == before {
		return nil
	}
	if err := r.cfg.SaveUser(ctx, user); err != nil {
		return fmt.Errorf("saving user %d: %w", user.ChatID, err)
	}
	return nil
}

// routeCallback runs the handler for a button press unless it comes from a stale menu.
func (r *Router) routeCallback(ctx context.Context, u *telegram.Update, user *User) error {
	action, args := decodeCallback(u.Text)
	cc := CallbackContext{
		MessageID: u.MessageID,
		Action:    action,
		Args:      args,
	}

	// Telegram keeps old inline keyboards pressable, so stale callbacks cannot take tracking.
	if u.MessageID != 0 && user.MenuMessageID != 0 && u.MessageID != user.MenuMessageID {
		return r.cfg.OnUnknownCallback(ctx, cc, user)
	}

	// A callback is adopted only when no navigation message is tracked or it is the tracked one.
	if u.MessageID != 0 {
		user.MenuMessageID = u.MessageID // route saves it with the user, so it survives restarts
	}

	if handler, ok := r.cfg.CallbackHandlers[action]; ok {
		return handler(ctx, cc, user)
	}
	return r.cfg.OnUnknownCallback(ctx, cc, user)
}

// routeMessage runs the handler for typed text, chosen by the user's state.
func (r *Router) routeMessage(ctx context.Context, u *telegram.Update, user *User) error {
	mc := MessageContext{
		MessageID: u.MessageID,
		Text:      u.Text,
	}

	if handler, ok := r.cfg.MessageHandlers[user.State]; ok {
		return handler(ctx, mc, user)
	}
	return r.cfg.OnUnknownMessage(ctx, mc, user)
}
