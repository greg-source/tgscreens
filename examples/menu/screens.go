package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/greg-source/tgscreens"
)

const pageSize = 5

// mainKeyboard is a reply keyboard; Telegram renders each inner slice as one row.
var mainKeyboard = [][]string{
	{btnNew},
	{btnList},
	{btnSettings},
}

// Screen builders are pure: given the data, they return what to display.

func writeScreen(notice string) tgscreens.Screen {
	return tgscreens.Screen{
		Text:    joinLines(notice, txtWritePrompt),
		Buttons: [][]tgscreens.Button{{cancelButton()}},
	}
}

func whenScreen(text string) tgscreens.Screen {
	return tgscreens.Screen{
		Text: fmt.Sprintf(txtWhenPrompt, text),
		Buttons: [][]tgscreens.Button{
			{whenButton(btnIn1Minute, time.Minute), whenButton(btnIn10Minutes, 10*time.Minute)},
			{whenButton(btnIn1Hour, time.Hour), whenButton(btnTomorrow, 24*time.Hour)},
			{cancelButton()},
		},
	}
}

// whenButton carries the delay in minutes: a small integer, never the
// reminder's text, which waits in the store as a draft.
func whenButton(caption string, after time.Duration) tgscreens.Button {
	return tgscreens.Button{Text: caption, Data: tgscreens.EncodeCallback(actWhen, strconv.Itoa(int(after.Minutes())))}
}

func cancelButton() tgscreens.Button {
	return tgscreens.Button{Text: btnCancel, Data: tgscreens.EncodeCallback(actCancel)}
}

func reminderSetScreen(r reminder) tgscreens.Screen {
	return tgscreens.Screen{
		Text: fmt.Sprintf(txtReminderSet, r.Text, r.Due.Format(dateFormat)),
		Buttons: [][]tgscreens.Button{{
			{Text: btnNew, Data: tgscreens.EncodeCallback(actNew)},
			{Text: btnList, Data: tgscreens.EncodeCallback(actList, "0")},
		}},
	}
}

func listScreen(reminders []reminder, paused bool, status string, page int) tgscreens.Screen {
	newRow := []tgscreens.Button{{Text: btnNew, Data: tgscreens.EncodeCallback(actNew)}}
	if len(reminders) == 0 {
		return tgscreens.Screen{
			Text:    joinLines(status, txtNoReminders),
			Buttons: [][]tgscreens.Button{newRow},
		}
	}

	labels := make([]string, len(reminders))
	for i, r := range reminders {
		labels[i] = fmt.Sprintf(txtListLabel, r.Due.Format(labelFormat), r.Text)
	}

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
}

func reminderScreen(r reminder, page int) tgscreens.Screen {
	return tgscreens.Screen{
		Text: fmt.Sprintf(txtReminder, r.Text, r.Due.Format(dateFormat)),
		Buttons: [][]tgscreens.Button{
			{{Text: btnDelete, Data: tgscreens.EncodeCallback(actDelete, strconv.Itoa(r.ID), strconv.Itoa(page))}},
			{{Text: btnBack, Data: tgscreens.EncodeCallback(actList, strconv.Itoa(page))}},
		},
	}
}

// settingsScreen shows the current state in the text and the change on the
// button. The button carries the target value rather than "toggle", so pressing
// it twice, or from an old screen, cannot flip the setting back.
func settingsScreen(paused bool, status string) tgscreens.Screen {
	state := txtRemindersOn
	toggle := tgscreens.Button{Text: btnPause, Data: tgscreens.EncodeCallback(actPause, "1")}
	if paused {
		state = txtRemindersPaused
		toggle = tgscreens.Button{Text: btnResume, Data: tgscreens.EncodeCallback(actPause, "0")}
	}

	return tgscreens.Screen{
		Text: joinLines(txtSettings, status, state),
		Buttons: [][]tgscreens.Button{
			{toggle},
			{{Text: btnClear, Data: tgscreens.EncodeCallback(actClear)}},
		},
	}
}

// clearScreen asks for confirmation; cancel returns to Settings.
func clearScreen() tgscreens.Screen {
	return tgscreens.Screen{
		Text: txtClearConfirm,
		Buttons: [][]tgscreens.Button{
			{{Text: btnClearOK, Data: tgscreens.EncodeCallback(actClearOK)}},
			{{Text: btnCancel, Data: tgscreens.EncodeCallback(actSettings)}},
		},
	}
}

// joinLines separates the non-empty parts with a blank line.
func joinLines(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "\n\n")
}
