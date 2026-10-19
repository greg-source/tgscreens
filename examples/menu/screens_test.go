package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/greg-source/tgscreens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListScreen_labelsAndCallbacks(t *testing.T) {
	due := time.Date(2026, 10, 20, 15, 30, 0, 0, time.UTC)

	s := listScreen([]reminder{{ID: 7, Text: "Buy milk", Due: due}}, true, "", 0)

	assert.Equal(t, "📋 Your reminders (1/1)\n\n🔕 Reminders are paused. Due ones will arrive when you resume.", s.Text)
	assert.Equal(t, [][]tgscreens.Button{
		{{Text: "20 Oct 15:30 · Buy milk", Data: "rem:7|0"}},
		{{Text: btnNew, Data: "new"}},
	}, s.Buttons)
}

func TestSettingsScreen_buttonCarriesTheTargetValue(t *testing.T) {
	on := settingsScreen(false, "")
	assert.Equal(t, "⚙️ Settings\n\n🔔 Reminders are on.", on.Text)
	assert.Equal(t, tgscreens.Button{Text: btnPause, Data: "pause:1"}, on.Buttons[0][0])

	paused := settingsScreen(true, "")
	assert.Equal(t, tgscreens.Button{Text: btnResume, Data: "pause:0"}, paused.Buttons[0][0])
}

func TestWhenScreen_delaysInMinutes(t *testing.T) {
	s := whenScreen("Buy milk")

	assert.Equal(t, "⏰ When should I remind you?\n\n«Buy milk»", s.Text)
	assert.Equal(t, [][]tgscreens.Button{
		{{Text: btnIn1Minute, Data: "when:1"}, {Text: btnIn10Minutes, Data: "when:10"}},
		{{Text: btnIn1Hour, Data: "when:60"}, {Text: btnTomorrow, Data: "when:1440"}},
		{{Text: btnCancel, Data: "cancel"}},
	}, s.Buttons)
}

// Telegram rejects the whole keyboard when one callback exceeds the cap, so the
// worst realistic screen is measured rather than assumed.
func TestListScreen_callbacksFitTelegramBudget(t *testing.T) {
	var reminders []reminder
	for i := range 1000 * pageSize {
		reminders = append(reminders, reminder{
			ID:   1_000_000_000 + i,
			Text: fmt.Sprintf("a long reminder that would overflow the budget if sent raw: %d", i),
		})
	}

	var buttons [][]tgscreens.Button
	buttons = append(buttons, listScreen(reminders, false, "", 999).Buttons...)
	buttons = append(buttons, reminderScreen(reminders[len(reminders)-1], 999).Buttons...)

	require.NotEmpty(t, buttons)
	for _, row := range buttons {
		for _, b := range row {
			assert.LessOrEqual(t, len(b.Data), tgscreens.MaxCallbackDataBytes, "callback %q", b.Data)
		}
	}
}
