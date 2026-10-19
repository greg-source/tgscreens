// Package telegram is the Telegram Bot API connection that tgscreens bots run on:
// a thin client, and the two update sources that feed a router.
//
// [Client] sends, edits and deletes messages and implements [Sender], the
// outbound contract the router depends on. [Poller] receives updates by long
// polling and [Webhook] receives them over HTTPS; each has a method that is a
// tgscreens.Source, so a bot switches between them by changing one line in its
// main function. Updates are translated into [Update] before they reach the
// router, so no signature a caller writes names the underlying library.
//
// The package knows nothing about dispatch, conversation state, or any bot's
// domain. It is also what a bot's tests replace: the telegram/mock package
// holds a generated mock of [Sender].
package telegram
