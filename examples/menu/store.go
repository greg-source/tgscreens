package main

import (
	"slices"
	"sync"
	"time"

	"github.com/greg-source/tgscreens"
)

type reminder struct {
	ID   int
	Text string
	Due  time.Time
}

// store keeps everything in memory; a real bot would reach its database here.
// Listen handles one update at a time, but reminders are delivered from a
// second goroutine, so every method locks.
type store struct {
	mu sync.Mutex

	users     map[int64]tgscreens.User
	reminders map[int64][]reminder // per chat, soonest first
	drafts    map[int64]string     // reminder text waiting for a time
	paused    map[int64]bool
	lastID    int
}

func newStore() *store {
	return &store{
		users:     make(map[int64]tgscreens.User),
		reminders: make(map[int64][]reminder),
		drafts:    make(map[int64]string),
		paused:    make(map[int64]bool),
	}
}

func (s *store) user(chatID int64) (tgscreens.User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[chatID]
	return user, ok
}

func (s *store) saveUser(user tgscreens.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[user.ChatID] = user
}

func (s *store) add(chatID int64, text string, due time.Time) reminder {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastID++
	r := reminder{ID: s.lastID, Text: text, Due: due}
	s.reminders[chatID] = append(s.reminders[chatID], r)
	slices.SortStableFunc(s.reminders[chatID], func(a, b reminder) int { return a.Due.Compare(b.Due) })
	return r
}

func (s *store) list(chatID int64) []reminder {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.reminders[chatID])
}

func (s *store) find(chatID int64, id int) (reminder, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.reminders[chatID], func(r reminder) bool { return r.ID == id })
	if i < 0 {
		return reminder{}, false
	}
	return s.reminders[chatID][i], true
}

func (s *store) remove(chatID int64, id int) (reminder, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.reminders[chatID], func(r reminder) bool { return r.ID == id })
	if i < 0 {
		return reminder{}, false
	}
	r := s.reminders[chatID][i]
	s.reminders[chatID] = slices.Delete(s.reminders[chatID], i, i+1)
	return r, true
}

func (s *store) clear(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.reminders, chatID)
}

func (s *store) setDraft(chatID int64, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drafts[chatID] = text
}

// takeDraft returns and forgets the chat's draft, so a time button pressed
// twice creates one reminder.
func (s *store) takeDraft(chatID int64) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	text, ok := s.drafts[chatID]
	delete(s.drafts, chatID)
	return text, ok
}

func (s *store) setPaused(chatID int64, paused bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused[chatID] = paused
}

func (s *store) isPaused(chatID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused[chatID]
}

type dueReminder struct {
	chatID int64
	reminder
}

// takeDue removes and returns the reminders due at now in chats that have not
// paused them. Removing before sending delivers each reminder at most once.
func (s *store) takeDue(now time.Time) []dueReminder {
	s.mu.Lock()
	defer s.mu.Unlock()

	var due []dueReminder
	for chatID, rs := range s.reminders {
		if s.paused[chatID] {
			continue
		}
		n := 0
		for n < len(rs) && !rs[n].Due.After(now) {
			due = append(due, dueReminder{chatID: chatID, reminder: rs[n]})
			n++
		}
		s.reminders[chatID] = rs[n:]
	}
	return due
}
