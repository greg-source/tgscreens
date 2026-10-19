// Package tgscreens provides stateful screens and paged lists for Telegram
// bots.
//
// It turns a chat into a set of screens. Each user's conversation state lives
// in your storage, one navigation message per chat is tracked, paged
// inline-keyboard lists are rendered for you, and button callbacks are encoded
// within Telegram's 64-byte limit. Buttons keep working across restarts, and
// old buttons from earlier screens are detected instead of acted on. The
// package knows nothing about your domain or your storage, and nothing about
// how updates reach it; the sibling package telegram supplies the Bot API
// client and the long-polling and webhook update sources.
//
// # The five pieces
//
// A bot is one [Config] literal built from five pieces:
//
//   - States and actions: [State] says where a user is in the conversation and
//     [Action] says what a button does. Both are only ever map keys here, so
//     you choose their values.
//   - LoadUser and SaveUser: the only two functions that reach your storage.
//     They translate between your own record and [User].
//   - Screen builders: plain functions that return a [Screen]; with
//     [PagedList] and [EncodeCallback] they involve no I/O.
//   - Handlers: one [MessageHandlerFunc] per state and one
//     [CallbackHandlerFunc] per action.
//   - Hooks: fallbacks for unrecognised input, plus OnError and OnAction,
//     which are required.
//
// # Rules worth knowing up front
//
// Handlers never persist: they mutate the *[User] they are given and the
// [Router] saves it afterwards when it changed. Handlers never assign
// User.MenuMessageID: [Router.Show] and [Router.Keyboard] own the navigation
// message. Action names, state values and callback layout are wire formats,
// because buttons already delivered to users stay pressable; append, never
// rename. See the docs directory of the repository for the long form.
//
// # What it does not do
//
// There are no ready-made widgets such as date pickers or sliders, and no
// back-stack: a screen that wants a back button names its parent in the
// callback data.
//
// # Minimal bot
//
// The smallest runnable bot, one state and one button, is examples/echo. A
// bot with several states, a paged list and a reply keyboard is
// examples/menu, and examples/webhook serves the echo bot through a webhook.
package tgscreens
