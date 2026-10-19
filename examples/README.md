# Examples

Each example is a complete, runnable bot. Get a token from [@BotFather](https://t.me/BotFather),
then run one from the repository root.

## echo

The smallest bot: one state, one inline button, and long polling. Typed text is echoed back with a
**Ping** button, and each press edits the same message into the next **Pong** instead of sending a
new one.

```bash
TELEGRAM_TOKEN=123456:ABC... go run ./examples/echo
```

<p align="center">
  <img src="echo/demo.gif" alt="The echo example: Ping edits the same message into Pong #1, then Pong #2" width="540">
</p>

Source: [`echo/main.go`](echo/main.go)

## menu

A reminder bot, laid out the way a real bot is. Everything is kept in memory.

- **➕ New reminder:** type what to remember, then pick when with a button: in 1 minute, 10 minutes,
  1 hour, or tomorrow.
- **📋 My reminders:** a paged list. Open a reminder to see it or delete it.
- **⚙️ Settings:** pause or resume reminders with one toggle, or delete them all after a
  confirmation.

Typed messages are deleted as they are handled, so the chat shows one screen that changes in
place. Due reminders are delivered by a background loop, and paused ones wait until you resume.

```bash
TELEGRAM_TOKEN=123456:ABC... go run ./examples/menu
```

<p align="center">
  <img src="menu/demo.gif" alt="The reminder bot: creating a reminder, browsing the list, pausing reminders and deleting them all" width="720">
</p>

Source: [`main.go`](menu/main.go) wires the bot, [`handlers.go`](menu/handlers.go) reacts to
updates, [`screens.go`](menu/screens.go) builds what is shown, [`text.go`](menu/text.go) holds the
copy, and [`store.go`](menu/store.go) is the in-memory storage.