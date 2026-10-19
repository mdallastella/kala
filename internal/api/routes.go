package api

import (
	"net/http"
	"strconv"
	"time"

	"kala/internal/service"
	"kala/internal/tags"
)

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	const p = "/api/v1/"
	mux.Handle("POST "+p+"auth/register", s.handle(s.register))
	mux.Handle("POST "+p+"auth/login", s.handle(s.login))
	mux.Handle("POST "+p+"auth/refresh", s.handle(s.refresh))
	mux.Handle("POST "+p+"auth/logout", s.handle(s.logout))
	mux.Handle("GET "+p+"auth/config", s.handle(s.authConfig))

	mux.Handle("GET "+p+"me", s.authed(s.getMe))
	mux.Handle("PATCH "+p+"me", s.authed(s.patchMe))
	mux.Handle("GET "+p+"me/working-hours", s.authed(s.getWorkingHours))
	mux.Handle("PUT "+p+"me/working-hours", s.authed(s.putWorkingHours))

	mux.Handle("GET "+p+"tokens", s.authed(s.listTokens))
	mux.Handle("POST "+p+"tokens", s.authed(s.createToken))
	mux.Handle("DELETE "+p+"tokens/{id}", s.authed(s.revokeToken))

	mux.Handle("GET "+p+"tasks", s.authed(s.listTasks))
	mux.Handle("POST "+p+"tasks", s.authed(s.createTask))
	mux.Handle("POST "+p+"tasks/parse", s.authed(s.parseQuick))
	mux.Handle("POST "+p+"tasks/bulk", s.authed(s.bulk))
	mux.Handle("GET "+p+"tasks/{id}", s.authed(s.getTask))
	mux.Handle("PATCH "+p+"tasks/{id}", s.authed(s.patchTask))
	mux.Handle("DELETE "+p+"tasks/{id}", s.authed(s.deleteTask))
	mux.Handle("POST "+p+"tasks/{id}/restore", s.authed(s.restoreTask))
	mux.Handle("POST "+p+"tasks/{id}/timer/start", s.authed(s.startTimer))
	mux.Handle("POST "+p+"tasks/{id}/timer/stop", s.authed(s.stopTimer))
	mux.Handle("GET "+p+"trash", s.authed(s.listTrash))
	mux.Handle("DELETE "+p+"trash/{id}", s.authed(s.purgeTask))

	mux.Handle("GET "+p+"timers/active", s.authed(s.activeTimers))
	mux.Handle("POST "+p+"timers/start-new", s.authed(s.startNew))
	mux.Handle("POST "+p+"timers/stop-all", s.authed(s.stopAll))
	mux.Handle("GET "+p+"time-entries", s.authed(s.listEntries))
	mux.Handle("POST "+p+"time-entries", s.authed(s.createEntry))
	mux.Handle("PATCH "+p+"time-entries/{id}", s.authed(s.patchEntry))
	mux.Handle("DELETE "+p+"time-entries/{id}", s.authed(s.deleteEntry))
	mux.Handle("GET "+p+"reports/time", s.authed(s.timeReport))

	mux.Handle("GET "+p+"tags", s.authed(s.listTags))
	mux.Handle("PUT "+p+"tags", s.authed(s.putTagMeta))
	mux.Handle("POST "+p+"tags/rename", s.authed(s.renameTag))
	mux.Handle("DELETE "+p+"tags/{id}", s.authed(s.deleteTag))

	mux.Handle("GET "+p+"namespaces", s.authed(s.listNamespaces))
	mux.Handle("POST "+p+"namespaces", s.authed(s.createNamespace))
	mux.Handle("PUT "+p+"namespaces/{id}", s.authed(s.updateNamespace))
	mux.Handle("DELETE "+p+"namespaces/{id}", s.authed(s.deleteNamespace))

	mux.Handle("POST "+p+"reminders/{id}/ack", s.authed(s.ackReminder))
	mux.Handle("GET "+p+"digest/briefing", s.authed(s.briefing))
	mux.Handle("GET "+p+"digest/wrapup", s.authed(s.wrapup))
	mux.Handle("POST "+p+"digest/{kind}/seen", s.authed(s.digestSeen))

	mux.Handle("GET "+p+"events", s.authed(s.events))
	mux.Handle("GET "+p+"export", s.authed(s.export))
	mux.Handle("POST "+p+"import", s.authed(s.importData))

	mux.Handle(p, s.handle(func(http.ResponseWriter, *http.Request) error { return service.ErrNotFound }))

	if s.mcp != nil {
		mux.Handle("/mcp", s.mcpAuth(s.mcp))
	}
	mux.Handle("/", s.spa())
}

// --- auth ---

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Timezone string `json:"timezone,omitempty"`
}

type session struct {
	AccessToken string        `json:"access_token"`
	ExpiresAt   time.Time     `json:"expires_at"`
	User        *service.User `json:"user"`
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID int64, refresh string) error {
	if refresh == "" {
		var err error
		if refresh, err = s.svc.IssueRefresh(r.Context(), userID); err != nil {
			return err
		}
	}
	// Secure is only off in dev mode, for plain-HTTP localhost.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // see above
		Name: refreshCookie, Value: refresh, Path: "/api/v1/auth", MaxAge: int(service.RefreshTTL.Seconds()),
		HttpOnly: true, Secure: !s.cfg.Dev, SameSite: http.SameSiteStrictMode,
	})
	tok, exp, err := s.signer.Issue(userID)
	if err != nil {
		return err
	}
	me, err := s.svc.Me(r.Context(), userID)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, session{AccessToken: tok, ExpiresAt: exp, User: me})
}

func (s *Server) rateLimit(r *http.Request) error {
	if !s.limiter.allow(s.clientIP(r)) {
		return &service.Error{Code: "rate_limited", Message: "too many attempts, try again later"}
	}
	return nil
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) error {
	if err := s.rateLimit(r); err != nil {
		return err
	}
	var c credentials
	if err := decode(w, r, &c, 0); err != nil {
		return err
	}
	u, err := s.svc.Register(r.Context(), c.Email, c.Password, c.Timezone, s.cfg.AllowSignup)
	if err != nil {
		return err
	}
	return s.startSession(w, r, u.ID, "")
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	if err := s.rateLimit(r); err != nil {
		return err
	}
	var c credentials
	if err := decode(w, r, &c, 0); err != nil {
		return err
	}
	id, err := s.svc.Authenticate(r.Context(), c.Email, c.Password)
	if err != nil {
		return err
	}
	return s.startSession(w, r, id, "")
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) error {
	c, err := r.Cookie(refreshCookie)
	if err != nil {
		return &service.Error{Code: service.CodeAuth, Message: "no session"}
	}
	id, next, err := s.svc.RotateRefresh(r.Context(), c.Value)
	if err != nil {
		s.clearCookie(w)
		return err
	}
	return s.startSession(w, r, id, next)
}

func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is only off in dev mode
		Name: refreshCookie, Value: "", Path: "/api/v1/auth", MaxAge: -1,
		HttpOnly: true, Secure: !s.cfg.Dev, SameSite: http.SameSiteStrictMode})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(refreshCookie); err == nil {
		if err := s.svc.RevokeRefresh(r.Context(), c.Value); err != nil {
			return err
		}
	}
	s.clearCookie(w)
	return writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) authConfig(w http.ResponseWriter, r *http.Request) error {
	n, err := s.svc.UserCount(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]bool{"signup_open": n == 0 || s.cfg.AllowSignup})
}

// --- profile ---

func (s *Server) getMe(w http.ResponseWriter, r *http.Request) error {
	me, err := s.svc.Me(r.Context(), uid(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, me)
}

func (s *Server) patchMe(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Timezone string `json:"timezone"`
	}
	if err := decode(w, r, &in, 0); err != nil {
		return err
	}
	me, err := s.svc.UpdateTimezone(r.Context(), uid(r), in.Timezone)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, me)
}

func (s *Server) getWorkingHours(w http.ResponseWriter, r *http.Request) error {
	days, err := s.svc.WorkingHours(r.Context(), uid(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, days)
}

func (s *Server) putWorkingHours(w http.ResponseWriter, r *http.Request) error {
	var days []service.WorkingDay
	if err := decode(w, r, &days, 0); err != nil {
		return err
	}
	out, err := s.svc.SetWorkingHours(r.Context(), uid(r), days)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) listTokens(w http.ResponseWriter, r *http.Request) error {
	list, err := s.svc.ListAccessTokens(r.Context(), uid(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, list)
}

func (s *Server) createToken(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name string `json:"name"`
	}
	if err := decode(w, r, &in, 0); err != nil {
		return err
	}
	at, err := s.svc.CreateAccessToken(r.Context(), uid(r), in.Name)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, at)
}

func (s *Server) revokeToken(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.svc.RevokeAccessToken(r.Context(), uid(r), id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusNoContent, nil)
}

// --- tasks ---

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) error {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := s.svc.Tasks(r.Context(), uid(r), service.ListOptions{Query: r.URL.Query().Get("q"), Limit: limit})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, list)
}

func (s *Server) listTrash(w http.ResponseWriter, r *http.Request) error {
	list, err := s.svc.Tasks(r.Context(), uid(r), service.ListOptions{Query: r.URL.Query().Get("q"), Deleted: true})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, list)
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) error {
	var in service.TaskInput
	if err := decode(w, r, &in, 0); err != nil {
		return err
	}
	t, err := s.svc.CreateTask(r.Context(), uid(r), in)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, t)
}

func (s *Server) parseQuick(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Quick string `json:"quick"`
	}
	if err := decode(w, r, &in, 0); err != nil {
		return err
	}
	res, err := s.svc.ParseQuick(r.Context(), uid(r), in.Quick)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, res)
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	t, err := s.svc.Task(r.Context(), uid(r), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

func (s *Server) patchTask(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var p service.TaskPatch
	if err := decode(w, r, &p, 0); err != nil {
		return err
	}
	t, err := s.svc.UpdateTask(r.Context(), uid(r), id, p)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.svc.DeleteTask(r.Context(), uid(r), id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) restoreTask(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	t, err := s.svc.RestoreTask(r.Context(), uid(r), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

func (s *Server) purgeTask(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.svc.PurgeTask(r.Context(), uid(r), id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) bulk(w http.ResponseWriter, r *http.Request) error {
	var in service.BulkInput
	if err := decode(w, r, &in, 0); err != nil {
		return err
	}
	if err := s.svc.Bulk(r.Context(), uid(r), in); err != nil {
		return err
	}
	return writeJSON(w, http.StatusNoContent, nil)
}

// --- timers ---

func (s *Server) startTimer(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	t, err := s.svc.StartTimer(r.Context(), uid(r), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

func (s *Server) stopTimer(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	t, err := s.svc.StopTimer(r.Context(), uid(r), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

func (s *Server) startNew(w http.ResponseWriter, r *http.Request) error {
	var in service.TaskInput
	if err := decode(w, r, &in, 0); err != nil {
		return err
	}
	t, err := s.svc.StartTimerNewTask(r.Context(), uid(r), in)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, t)
}

func (s *Server) stopAll(w http.ResponseWriter, r *http.Request) error {
	n, err := s.svc.StopAllTimers(r.Context(), uid(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]int64{"stopped": n})
}

func (s *Server) activeTimers(w http.ResponseWriter, r *http.Request) error {
	list, err := s.svc.ActiveTimers(r.Context(), uid(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, list)
}

func (s *Server) entryFilter(r *http.Request) (service.EntryFilter, error) {
	q := r.URL.Query()
	now := s.svc.Now()
	f := service.EntryFilter{From: now.AddDate(0, 0, -7), To: now.Add(24 * time.Hour), Query: q.Get("q")}
	for name, dst := range map[string]*time.Time{"from": &f.From, "to": &f.To} {
		if v := q.Get(name); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return f, &service.Error{Code: service.CodeInvalid, Message: name + " must be an RFC 3339 timestamp"}
			}
			*dst = t
		}
	}
	if v := q.Get("task"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return f, &service.Error{Code: service.CodeInvalid, Message: "invalid task id"}
		}
		f.TaskID = id
	}
	return f, nil
}

func (s *Server) listEntries(w http.ResponseWriter, r *http.Request) error {
	f, err := s.entryFilter(r)
	if err != nil {
		return err
	}
	list, err := s.svc.TimeEntries(r.Context(), uid(r), f)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, list)
}

func (s *Server) createEntry(w http.ResponseWriter, r *http.Request) error {
	var in service.EntryInput
	if err := decode(w, r, &in, 0); err != nil {
		return err
	}
	e, err := s.svc.CreateTimeEntry(r.Context(), uid(r), in)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, e)
}

func (s *Server) patchEntry(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in service.EntryInput
	if err := decode(w, r, &in, 0); err != nil {
		return err
	}
	e, err := s.svc.UpdateTimeEntry(r.Context(), uid(r), id, in)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, e)
}

func (s *Server) deleteEntry(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.svc.DeleteTimeEntry(r.Context(), uid(r), id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) timeReport(w http.ResponseWriter, r *http.Request) error {
	f, err := s.entryFilter(r)
	if err != nil {
		return err
	}
	rep, err := s.svc.TimeReport(r.Context(), uid(r), f, r.URL.Query().Get("group"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, rep)
}

// --- tags, namespaces ---

func (s *Server) listTags(w http.ResponseWriter, r *http.Request) error {
	list, err := s.svc.Tags(r.Context(), uid(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, list)
}

func (s *Server) putTagMeta(w http.ResponseWriter, r *http.Request) error {
	var in service.TagMeta
	if err := decode(w, r, &in, 0); err != nil {
		return err
	}
	t, err := s.svc.SetTagMeta(r.Context(), uid(r), in)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

func (s *Server) renameTag(w http.ResponseWriter, r *http.Request) error {
	var in service.RenameInput
	if err := decode(w, r, &in, 0); err != nil {
		return err
	}
	prev, err := s.svc.RenameTag(r.Context(), uid(r), in)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, prev)
}

func (s *Server) deleteTag(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.svc.DeleteTag(r.Context(), uid(r), id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) listNamespaces(w http.ResponseWriter, r *http.Request) error {
	list, err := s.svc.Namespaces(r.Context(), uid(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, list)
}

func (s *Server) createNamespace(w http.ResponseWriter, r *http.Request) error {
	var in tags.Namespace
	if err := decode(w, r, &in, 0); err != nil {
		return err
	}
	ns, err := s.svc.CreateNamespace(r.Context(), uid(r), in)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, ns)
}

func (s *Server) updateNamespace(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in tags.Namespace
	if err := decode(w, r, &in, 0); err != nil {
		return err
	}
	ns, err := s.svc.UpdateNamespace(r.Context(), uid(r), id, in)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, ns)
}

func (s *Server) deleteNamespace(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.svc.DeleteNamespace(r.Context(), uid(r), id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusNoContent, nil)
}

// --- reminders & digests ---

func (s *Server) ackReminder(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		SnoozeUntil *time.Time `json:"snooze_until"`
	}
	if r.ContentLength != 0 {
		if err := decode(w, r, &in, 0); err != nil {
			return err
		}
	}
	t, err := s.svc.AckReminder(r.Context(), uid(r), id, in.SnoozeUntil)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

func (s *Server) briefing(w http.ResponseWriter, r *http.Request) error {
	b, err := s.svc.Briefing(r.Context(), uid(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, b)
}

func (s *Server) wrapup(w http.ResponseWriter, r *http.Request) error {
	b, err := s.svc.Wrapup(r.Context(), uid(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, b)
}

func (s *Server) digestSeen(w http.ResponseWriter, r *http.Request) error {
	if err := s.svc.MarkDigestSeen(r.Context(), uid(r), r.PathValue("kind")); err != nil {
		return err
	}
	return writeJSON(w, http.StatusNoContent, nil)
}

// --- export ---

func (s *Server) export(w http.ResponseWriter, r *http.Request) error {
	ex, err := s.svc.Export(r.Context(), uid(r))
	if err != nil {
		return err
	}
	w.Header().Set("Content-Disposition", `attachment; filename="kala-export-`+ex.ExportedAt.Format("2006-01-02")+`.json"`)
	return writeJSON(w, http.StatusOK, ex)
}

func (s *Server) importData(w http.ResponseWriter, r *http.Request) error {
	var ex service.Export
	if err := decode(w, r, &ex, 64<<20); err != nil {
		return err
	}
	res, err := s.svc.Import(r.Context(), uid(r), &ex)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, res)
}
