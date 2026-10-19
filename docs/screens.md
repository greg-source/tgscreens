# Screens and callbacks

A screen is a message with an inline keyboard. Building one involves no I/O, so screen builders
are plain functions of their inputs and are tested without mocks. This guide covers `Screen`,
`PagedList`, and the callback codec that keeps button data within Telegram's limit.

The snippets are taken from the reminder bot in [`examples/menu`](../examples/menu/screens.go).

## Screen

```text
type Screen struct {
	Text      string
	ParseMode telegram.ParseMode
	Buttons   [][]Button
}
```

`Buttons` is a list of rows; each inner slice is one row of inline buttons. A `Button` has a
caption, `Text`, and `Data`, the callback data returned when it is pressed. `Button` is an alias
of `telegram.Button`, so a screen builder does not need to import the transport package for it.

`ParseMode` is the zero value `telegram.ParseModePlain` unless you set `telegram.ParseModeHTML`,
which enables Telegram's supported HTML tags in `Text`. It does not format button captions or
reply keyboard labels. Escape any externally supplied text before inserting it into HTML.

A `Screen` with no `Buttons` renders a message with no keyboard; editing a screen to have none
removes the keyboard from the message.

Pass a screen to `Router.Show`. See [the guide](guide.md#the-navigation-message) for how it
edits or replaces the tracked navigation message.

## PagedList

`PagedList` renders a slice of labels as one page of inline buttons, one item per row, with
previous and next navigation and an optional back button:

| Field | Meaning |
| --- | --- |
| `Items` | The labels to page through. Each becomes a button caption. |
| `Page` | The zero-based page to render. Clamped into range. |
| `PageSize` | Items per page. A value below 1 is treated as 1. |
| `Data` | Returns the callback data for an item: `func(page, i int, item string) string`. `page` is the page actually rendered and `i` indexes `Items`. Required when `Items` is not empty. |
| `Nav` | Returns the callback data for a target page: `func(page int) string`. Required when there is more than one page. |
| `Header` | Rows rendered above the items. |
| `Footer` | Rows rendered below the pager and above Back. |
| `Back`, `BackText` | Callback data and caption of a back button. Omitted when `Back` is empty. |
| `PrevText`, `NextText` | Navigation captions. Default to ⬅️ and ➡️. |

`Build` returns the keyboard, the page it actually rendered, and the total page count. The page is
clamped, so a button from a list that has since shrunk lands on the last real page rather than an
empty one. Label the screen with the returned page, not the one you asked for:

```go
	buttons, page, totalPages := tgscreens.PagedList{
		Items:    labels,
		Page:     page,
		PageSize: pageSize,
		// The callback names a reminder by ID rather than row, so a deletion that
		// shifts the rows does not change what a tap opens.
		Data: func(page, i int, _ string) string {
			return tgscreens.EncodeCallback(actReminder, strconv.Itoa(reminders[i].ID), strconv.Itoa(page))
		},
		Nav: func(page int) string {
			return tgscreens.EncodeCallback(actList, strconv.Itoa(page))
		},
		Footer: [][]tgscreens.Button{newRow},
	}.Build()

	pausedNotice := ""
	if paused {
		pausedNotice = txtRemindersPaused
	}
	return tgscreens.Screen{
		Text:    joinLines(fmt.Sprintf(txtListHeader, page+1, totalPages), status, pausedNotice),
		Buttons: buttons,
	}
```

The navigation row contains Previous only when there is an earlier page and Next only when there
is a later one, and no row at all for a single page. For five items, a page size of two, page one,
a back button, and callbacks of the form `pick:<index>` and `list:<page>`, `Build` returns page 1 of
3 and these rows:

| Row | Buttons (caption, callback data) |
| --- | --- |
| 1 | three, `pick:2` |
| 2 | four, `pick:3` |
| 3 | ⬅️, `list:0` and ➡️, `list:2` |
| 4 | Back, `menu` |

## The callback codec

Telegram caps callback data at 64 bytes, and exceeding the cap fails the whole keyboard rather
than the offending button. `tgscreens.MaxCallbackDataBytes` is that constant. Callback data is
therefore a short action name followed by short opaque arguments.

`EncodeCallback(action, args...)` produces `action` when there are no arguments and
`action:arg|arg` otherwise, for example `menu` or `pick:<hash>|2`.

The format has three rules:

- An action must not contain `:`; the first `:` ends the action.
- An argument must not contain `|`, which separates arguments.
- Arguments must be short opaque tokens, such as a `Hash` or a small integer, never user text.
  Free text blows the byte budget and corrupts the encoding if it happens to contain a separator.

Decoding is forgiving: data with no `:` is an action with no arguments, and `menu:` is the action
`menu` with one empty argument. Garbage decodes to an action no handler is registered for, so it
reaches `OnUnknownCallback` rather than whichever handler sits at a zero value.

Handlers read arguments from the `CallbackContext`. `Arg(i)` returns `""` and `ArgInt(i)` returns `0` for a missing or non-numeric argument; neither
panics, so a malformed callback degrades into an unknown value instead of a crash.

## Hash

`Hash(value)` returns an eight-character, stable hash of a string: the first four bytes of its
SHA-256, hex encoded. It lets a button name an item without carrying the item's text. The handler
resolves the hash against the list the user is looking at with `FindByHash`, which returns the
first item whose hash matches and reports a miss when none does.

Always handle the miss. The list may have changed since the button was drawn, and a hash that no
longer resolves must show the list again, not act on a wrong item. The menu example follows the
same rule for reminder IDs in `onReminder`.

Eight hex characters are 32 bits, which is plenty to tell apart the items of one list and not a
global identifier. Two items of the same list that collide resolve to the first one.

## The 64-byte budget in practice

A Cyrillic or emoji-heavy label costs two to four bytes per character, so a name that looks short
can overflow. Hashing is the fix, and a test is the guard: measure the worst realistic screen
rather than assume. The menu example asserts every button of a full page:

```go
	require.NotEmpty(t, buttons)
	for _, row := range buttons {
		for _, b := range row {
			assert.LessOrEqual(t, len(b.Data), tgscreens.MaxCallbackDataBytes, "callback %q", b.Data)
		}
	}
```

## Wire-format rules

Callback data, action names and persisted state values outlive the process that wrote them:
buttons already delivered stay pressable until the user deletes the chat.

- Append new actions; never rename or reuse a retired one.
- Never reorder or renumber state values you persist.
- Keep arguments positional and append new ones at the end. An older button must still decode.
- Reply keyboard captions are matched as literal text. Changing one breaks recognition for every
  client that still has the old keyboard cached, so keep an alias for the old caption.
