# Guide

tgscreens turns a Telegram chat into a set of screens: it turns a stream of updates into calls to
your handlers, and keeps the chat tidy while it does. This guide explains the model and how to
connect it to Telegram. [Screens and callbacks](screens.md) covers what handlers render.

The snippets are taken from the runnable bots in [`examples/`](../examples), the smallest being
[`examples/echo`](../examples/echo/main.go), so the code is compiled and tested along with the
library.

## What a bot supplies

A bot is one `tgscreens.Config` literal, built from five pieces:

1. **States and actions** — where a user is, and what a button does.
2. **`LoadUser` and `SaveUser`** — the only two functions that reach your storage.
3. **Screen builders** — plain functions that return a `Screen`. See [screens](screens.md).
4. **Handlers** — one per state for typed text, one per action for button presses.
5. **Hooks** — fallbacks for unrecognised input, plus `OnError` and `OnAction`.

`tgscreens.New` panics when `LoadUser`, `SaveUser`, `OnUnknownMessage`, `OnUnknownCallback`,
`OnError` or `OnAction` is missing, or when the sender is nil. These are wiring mistakes, caught
at startup, so `New` returns no error.

## States and actions

`State` is an `int` and `Action` is a `string`. The router only ever uses them as map keys, so
their values are yours to choose, and a state can carry whatever persisted meaning your storage
already gives it:

```go
const (
	stateMain tgscreens.State  = 0
	actPing   tgscreens.Action = "ping"
)
```

Treat both as wire formats. A state value is usually persisted, and an action name sits in
callback data on buttons that were delivered to users and stay pressable forever. Append new
values; never rename or renumber old ones. An action name also travels in callback data, so
keep it short: it counts against the [64-byte budget](screens.md#the-64-byte-budget-in-practice).

## Users and persistence

`User` is the per-user state the router needs, and nothing more:

| Field | Meaning |
| --- | --- |
| `ChatID` | The chat the update came from, and where replies go. |
| `Username` | The sender's Telegram username; may be empty. |
| `State` | Selects the message handler for typed text. |
| `MenuMessageID` | The tracked navigation message, or `0` when none is tracked. |

Your bot maps its own record onto `User` in `LoadUser` and back in `SaveUser`. The example keeps
users in a map; a real bot would call its database here:

```go
func (b *bot) loadUser(_ context.Context, username string, chatID int64) (tgscreens.User, error) {
	if user, ok := b.users[chatID]; ok {
		return user, nil
	}
	return tgscreens.User{ChatID: chatID, Username: username, State: stateMain}, nil
}

func (b *bot) saveUser(_ context.Context, user tgscreens.User) error {
	b.users[user.ChatID] = user
	return nil
}
```

`LoadUser` runs once per update and should return the existing user, creating one on first
contact. `SaveUser` runs only when the user changed during the update, which the router detects by
comparing the loaded and final values with `==`. That is why `User` must stay comparable:
`user.go` carries a compile-time guard that fails if a non-comparable field is ever added.

If your record has other columns that are written elsewhere, have `SaveUser` write only the
fields above, so a save cannot overwrite them with stale values.

## One update, step by step

```text
update arrives
  LoadUser                                   failure: OnError(nil user)
  dispatch
    typed text   -> MessageHandlers[user.State], else OnUnknownMessage
    button press -> CallbackHandlers[action], else OnUnknownCallback
  SaveUser, if the user changed
  on any error so far: OnError, then SaveUser again if the hook changed the user
  OnAction, always last
```

Updates are handled one at a time, in the order the source delivers them, so handlers never run
concurrently within one router.

Two rules follow from this flow:

**The router saves, not your handlers.** A handler mutates the `*User` it receives, for example
`user.State = stateAdding`, and returns. Afterwards the router compares that user with the one
`LoadUser` returned and, if it changed, calls your `SaveUser`. That call is the only write, and it
covers the navigation message too: `Show` puts the new message's ID in `user.MenuMessageID`, and
the router saves it with the rest of the user. The save happens even when the handler returned an
error, so a state change made before a failure is kept. If handling or that first save failed,
`OnError` runs with the user as the handler left it and may show a retry screen; the router then
saves whatever the hook changed.

**Handlers never touch `MenuMessageID`.** `Show` and `Keyboard` own it. See the next section.

## The navigation message

A chat is easy to litter: every menu is a new message, and the old ones stay on screen with
buttons that still work. tgscreens tracks one navigation message per chat, `User.MenuMessageID`,
and the router methods maintain it:

| Method | Does |
| --- | --- |
| `Show(ctx, user, editMessageID, screen)` | With a non-zero `editMessageID`, edits that message in place. With `0`, deletes the tracked message, sends the screen as a new one, and tracks it. |
| `Keyboard(ctx, user, text, keyboard)` | Deletes the tracked message, then sends text under a persistent reply keyboard. |
| `SendKeyboard(ctx, user, text, keyboard)` | Sends the reply keyboard without changing the tracked message. |
| `Send(ctx, chatID, text)` | Sends plain text without changing the tracked message. |
| `Discard(ctx, chatID, messageID)` | Best-effort deletes an incoming message; `0` is ignored. |

Callback handlers pass `cc.MessageID`, the message the pressed button sits on, so the screen is
rewritten in place. Message handlers pass `0`, because a typed message has nothing to edit:

```go
func (b *bot) onMessage(ctx context.Context, mc tgscreens.MessageContext, user *tgscreens.User) error {
	return b.Show(ctx, user, 0, tgscreens.Screen{
		Text:    "You said: " + mc.Text,
		Buttons: pingButton(1),
	})
}

func (b *bot) onPing(ctx context.Context, cc tgscreens.CallbackContext, user *tgscreens.User) error {
	n := cc.ArgInt(0)
	return b.Show(ctx, user, cc.MessageID, tgscreens.Screen{
		Text:    fmt.Sprintf("Pong #%d", n),
		Buttons: pingButton(n + 1),
	})
}
```

Passing the wrong value has visible effects: a message handler that passes a stale ID edits a
message unrelated to the update, and a callback handler that passes `0` sends a second menu.

Deleting the old message is best effort. Telegram refuses to delete messages older than 48 hours,
and the user may have removed it already. A failed delete is reported through the optional
`OnCleanupError` hook, never as an error of the update, and the tracked ID is cleared either way.

A handler that assigns `MenuMessageID` directly can leave a message that is remembered but was
never shown, or forgotten but never deleted, which strands it in the chat for good.

## Stale callbacks

Telegram keeps old inline keyboards pressable. When a button is pressed and the router already
tracks a different navigation message, the press comes from an older screen. The router does not
adopt it and does not run the action: it goes to `OnUnknownCallback` instead, so an old screen
cannot take over the session or run an action against a state it no longer represents.

The same hook receives a press whose action has no handler, typically a button left behind by an
older release. Both cases usually want the same answer, which is to do nothing:

```go
// ignoreCallback answers a stale or unknown button quietly instead of redrawing
// the message it sits on.
func ignoreCallback(context.Context, tgscreens.CallbackContext, *tgscreens.User) error { return nil }
```

Redrawing the old message, or sending a fresh menu, are alternatives, but a quiet answer keeps the
chat from jumping around when someone taps something from last week. The bundled update sources
answer every callback query before it is routed, so a quiet answer does not leave the button's
loading spinner running.

## Restarts

Nothing about a screen lives in the process. The navigation message ID is part of the `User` you
persist, and callback data carries its own action and arguments, so a button keeps working after a
restart or a redeploy, and a button from an earlier screen is still recognised as stale.

This holds only as far as your storage does. If `SaveUser` writes to memory, a restart forgets
which message is the menu, and every old button is then treated as current.

## Hooks

| Hook | Required | Called |
| --- | --- | --- |
| `OnUnknownMessage` | yes | For typed text in a state with no entry in `MessageHandlers`. |
| `OnUnknownCallback` | yes | For a stale press, or an action with no entry in `CallbackHandlers`. |
| `OnError` | yes | After a failed update. `user` is `nil` only when `LoadUser` failed. |
| `OnAction` | yes | After every update, last, whatever it did and however it went. |
| `OnCleanupError` | no | When a best-effort message deletion fails. |

`OnError` is where the user hears that something failed. With a non-nil user, replace the
navigation with a retry notice using `Show`; with a nil user nothing is tracked, so use `Send`.
The [menu example](../examples/menu/main.go) does exactly this in `onError`.

`OnAction` receives the user as `LoadUser` returned it, before the handler changed anything, the
raw text or callback data, whether it was a button press, and the update's final error. The
package keeps no record of its own, so this is the place to write an audit log or collect metrics.
It must not fail the update; log and return.

Both `OnError` and `OnAction` are required on purpose. A bot that has nothing to record still has
to say so, which makes silently losing failures impossible.

## Connecting to Telegram

The `telegram` package holds the Bot API client and the two ways updates arrive. `telegram.New`
validates the bot token and returns a `Client`, which implements `telegram.Sender`, the one
outbound dependency `tgscreens.New` takes. An update source is a `tgscreens.Source` that you pass
to `Listen`. Both sources deliver only typed messages and button presses, and answer each button
press right away.

Long polling needs nothing publicly reachable. Clear any webhook first, or Telegram refuses to
deliver updates by polling:

```go
	// Long polling fails with a conflict while a webhook is registered.
	if err := client.DeleteWebhook(ctx); err != nil {
		log.Fatal(err)
	}

	b := newBot(client)
	if err := b.Listen(ctx, telegram.NewPoller(client).Poll); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
```

A webhook makes Telegram post each update to an HTTPS URL you serve:

```go
	wh, err := telegram.NewWebhook(client, webhookURL, certFile, keyFile)
	if err != nil {
		log.Fatal(err)
	}
	if err := wh.Register(); err != nil {
		log.Fatal(err)
	}

	b := newBot(client)
	if err := b.Listen(ctx, wh.Listen); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
```

`NewWebhook` accepts only an `https` URL that has a path and uses port 443, 80, 88 or 8443 (443 when
omitted). The path is the only authentication, so make it unguessable. `Register` uploads the
certificate, so a self-signed one works, and asks for one connection at a time so updates arrive
in order. The process terminates TLS itself with the certificate and key files, and when the
context is cancelled it shuts down and deletes the webhook. The complete program is
[`examples/webhook`](../examples/webhook/main.go).
