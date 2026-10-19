package main

// Every user-facing string the bot can produce, so its voice can be read and
// changed in one place.

// Reply-keyboard captions. They come back as typed text and are matched
// literally, so changing one breaks every client that still shows the old keyboard.
const (
	btnNew      = "➕ New reminder"
	btnList     = "📋 My reminders"
	btnSettings = "⚙️ Settings"
)

const cmdStart = "/start"

// Inline-button captions.
const (
	btnIn1Minute   = "⏱ In 1 minute"
	btnIn10Minutes = "⏱ In 10 minutes"
	btnIn1Hour     = "🕐 In 1 hour"
	btnTomorrow    = "🌅 Tomorrow"
	btnCancel      = "❌ Cancel"
	btnBack        = "🔙 Back"
	btnDelete      = "🗑 Delete"
	btnPause       = "🔕 Pause reminders"
	btnResume      = "🔔 Resume reminders"
	btnClear       = "🗑 Delete all reminders"
	btnClearOK     = "🗑 Yes, delete all"
)

const (
	txtWelcome = "👋 Hi! I'll remind you about anything you like.\n\nPick an option below 👇"
	txtError   = "⚠️ Something went wrong. Please try again."
)

const (
	txtWritePrompt = "✍️ What should I remind you about?"
	txtTextEmpty   = "⚠️ Send the reminder as text."
	txtTextTooLong = "⚠️ That reminder is too long."
	txtWhenPrompt  = "⏰ When should I remind you?\n\n«%s»"
	txtDraftLost   = "⚠️ That reminder was not saved. Start a new one."
	txtReminderSet = "✅ Reminder set\n\n«%s»\n📅 %s"
)

const (
	txtNoReminders  = "📭 You have no reminders yet."
	txtListHeader   = "📋 Your reminders (%d/%d)"
	txtListLabel    = "%s · %s"
	txtReminder     = "⏰ «%s»\n📅 %s"
	txtDeleted      = "🗑 Deleted «%s»."
	txtReminderGone = "🔍 That reminder is gone."
)

const (
	txtSettings        = "⚙️ Settings"
	txtRemindersOn     = "🔔 Reminders are on."
	txtRemindersPaused = "🔕 Reminders are paused. Due ones will arrive when you resume."
	txtClearConfirm    = "🗑 Delete all your reminders?\n\nThis cannot be undone."
	txtAllDeleted      = "✅ All reminders deleted."
)

// txtDelivery is the reminder itself, sent as a plain message outside navigation.
const txtDelivery = "⏰ Reminder\n\n%s"

const (
	dateFormat  = "Mon 2 Jan, 15:04"
	labelFormat = "2 Jan 15:04"
)
