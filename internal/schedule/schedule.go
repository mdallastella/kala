// Package schedule pushes reminders and daily digests to connected users and
// runs periodic maintenance.
package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"kala/internal/dates"
	"kala/internal/events"
	"kala/internal/service"
)

// Interval is how often the scheduler checks for due notifications.
const Interval = 30 * time.Second

// Scheduler evaluates reminders and digests for users with open clients.
type Scheduler struct {
	svc *service.Service
	hub *events.Hub
	log *slog.Logger

	mu sync.Mutex
	// sentReminders remembers which reminder (task → remind_at) was pushed,
	// so that every tick does not repeat it.
	sentReminders map[int64]time.Time
	// sentDigests remembers "user/date/kind" digests already pushed.
	sentDigests map[string]bool
	lastPurge   time.Time
}

// New creates a scheduler and hooks catch-up delivery into the hub.
func New(svc *service.Service, hub *events.Hub, log *slog.Logger) *Scheduler {
	s := &Scheduler{svc: svc, hub: hub, log: log, sentReminders: map[int64]time.Time{}, sentDigests: map[string]bool{}}
	hub.OnSubscribe(s.catchUp)
	return s
}

// Run ticks until ctx is done.
func (s *Scheduler) Run(ctx context.Context) {
	t := time.NewTicker(Interval)
	defer t.Stop()
	s.Tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Tick(ctx)
		}
	}
}

// DigestDue returns the digest kind that should be shown now, or "".
// The briefing is due during working hours, the wrap-up after they end; each
// at most once per day.
func DigestDue(st *service.DigestState, now time.Time) string {
	today := dates.On(now, st.Cal.Loc)
	h, ok := st.Cal.Hours(today)
	if !ok {
		return ""
	}
	day := today.String()
	start, end := today.At(h.Start, st.Cal.Loc), today.At(h.End, st.Cal.Loc)
	switch {
	case !now.Before(end):
		if st.LastWrapup != day {
			return service.DigestWrapup
		}
	case !now.Before(start):
		if st.LastBriefing != day {
			return service.DigestBriefing
		}
	}
	return ""
}

// Tick runs one evaluation round.
func (s *Scheduler) Tick(ctx context.Context) {
	now := s.svc.Now()
	for _, uid := range s.hub.Users() {
		if err := s.tickUser(ctx, uid, now); err != nil {
			s.log.Error("scheduler", "user", uid, "err", err)
		}
	}
	s.mu.Lock()
	purge := now.Sub(s.lastPurge) > 24*time.Hour
	if purge {
		s.lastPurge = now
		for id, at := range s.sentReminders {
			if now.Sub(at) > 48*time.Hour {
				delete(s.sentReminders, id)
			}
		}
	}
	s.mu.Unlock()
	if purge {
		if err := s.svc.PurgeExpired(ctx); err != nil {
			s.log.Error("purge", "err", err)
		}
	}
}

func (s *Scheduler) tickUser(ctx context.Context, uid int64, now time.Time) error {
	due, err := s.svc.DueReminders(ctx, uid)
	if err != nil {
		return err
	}
	for _, t := range due {
		s.mu.Lock()
		sent, ok := s.sentReminders[t.ID]
		fresh := !ok || !sent.Equal(*t.RemindAt)
		if fresh {
			s.sentReminders[t.ID] = *t.RemindAt
		}
		s.mu.Unlock()
		if fresh {
			s.hub.Publish(uid, events.Event{Type: "reminder", Data: t})
		}
	}
	st, err := s.svc.DigestState(ctx, uid)
	if err != nil {
		return err
	}
	if kind := DigestDue(st, now); kind != "" {
		key := fmt.Sprintf("%d/%s/%s", uid, dates.On(now, st.Cal.Loc), kind)
		s.mu.Lock()
		fresh := !s.sentDigests[key]
		s.sentDigests[key] = true
		s.mu.Unlock()
		if fresh {
			s.hub.Publish(uid, events.Event{Type: kind})
		}
	}
	return nil
}

// catchUp sends a new client the reminders and digest it missed.
func (s *Scheduler) catchUp(uid int64, ch chan<- events.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	send := func(e events.Event) {
		select {
		case ch <- e:
		default:
		}
	}
	due, err := s.svc.DueReminders(ctx, uid)
	if err != nil {
		s.log.Error("catch-up reminders", "user", uid, "err", err)
		return
	}
	for _, t := range due {
		send(events.Event{Type: "reminder", Data: t})
	}
	st, err := s.svc.DigestState(ctx, uid)
	if err != nil {
		s.log.Error("catch-up digest", "user", uid, "err", err)
		return
	}
	if kind := DigestDue(st, s.svc.Now()); kind != "" {
		send(events.Event{Type: kind})
	}
}
