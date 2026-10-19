package service

import (
	"context"
	"database/sql"
	"errors"
	"net/mail"
	"strings"
	"time"

	"kala/internal/auth"
	"kala/internal/dates"
	"kala/internal/tags"
)

// RefreshTTL is the lifetime of a refresh token.
const RefreshTTL = 30 * 24 * time.Hour

// User is the public profile of an account.
type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Timezone  string    `json:"timezone"`
	CreatedAt time.Time `json:"created_at"`
}

// UserCount returns the number of registered users.
func (s *Service) UserCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&n)
	return n, err
}

func validatePassword(pw string) error {
	if len(pw) < 8 {
		return errf(CodeInvalid, "password must be at least 8 characters")
	}
	if len(pw) > 72 {
		return errf(CodeInvalid, "password must be at most 72 bytes")
	}
	return nil
}

func validateTimezone(tz string) error {
	if tz == "" {
		return errf(CodeInvalid, "timezone is required")
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return errf(CodeInvalid, "unknown timezone %q", tz)
	}
	return nil
}

// Register creates a user with the default namespaces, views and working
// hours. allowSignup is consulted only when users already exist.
func (s *Service) Register(ctx context.Context, email, password, timezone string, allowSignup bool) (*User, error) {
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != strings.TrimSpace(email) {
		return nil, errf(CodeInvalid, "invalid email address")
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	if timezone == "" {
		timezone = "UTC"
	}
	if err := validateTimezone(timezone); err != nil {
		return nil, err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	var id int64
	now := ts(s.now())
	err = s.tx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&n); err != nil {
			return err
		}
		if n > 0 && !allowSignup {
			return errf(CodeDenied, "registration is closed")
		}
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE email = ?", addr.Address).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			return errf(CodeConflict, "email already registered")
		}
		if err := tx.QueryRowContext(ctx,
			"INSERT INTO users (email, password_hash, timezone, created_at) VALUES (?, ?, ?, ?) RETURNING id",
			addr.Address, hash, timezone, now).Scan(&id); err != nil {
			return err
		}
		return seedUser(ctx, tx, id)
	})
	if err != nil {
		return nil, err
	}
	return s.Me(ctx, id)
}

// DefaultViews are seeded for every user.
var DefaultViews = []struct{ Name, Query string }{
	{"Today", "is:open due:<=today"},
	{"Upcoming", "is:open due:week sort:due,priority"},
	{"Overdue", "is:open due:overdue sort:due"},
	{"Inbox", "is:open -project:"},
	{"All open", "is:open"},
}

func seedUser(ctx context.Context, tx *sql.Tx, userID int64) error {
	for _, ns := range tags.Seed() {
		if _, err := insertNamespace(ctx, tx, userID, ns); err != nil {
			return err
		}
	}
	for i, v := range DefaultViews {
		if _, err := tx.ExecContext(ctx, "INSERT INTO views (user_id, name, query, position, builtin) VALUES (?, ?, ?, ?, 1)",
			userID, v.Name, v.Query, i); err != nil {
			return err
		}
	}
	for wd := time.Monday; wd <= time.Friday; wd++ {
		if _, err := tx.ExecContext(ctx, "INSERT INTO working_hours (user_id, weekday, start_min, end_min) VALUES (?, ?, ?, ?)",
			userID, int(wd), 9*60, 18*60); err != nil {
			return err
		}
	}
	return nil
}

// Authenticate checks credentials and returns the user id.
func (s *Service) Authenticate(ctx context.Context, email, password string) (int64, error) {
	var (
		id   int64
		hash string
	)
	err := s.db.QueryRowContext(ctx, "SELECT id, password_hash FROM users WHERE email = ?", strings.TrimSpace(email)).Scan(&id, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		auth.BurnTime(password)
		return 0, errf(CodeAuth, "invalid email or password")
	}
	if err != nil {
		return 0, err
	}
	if !auth.CheckPassword(hash, password) {
		return 0, errf(CodeAuth, "invalid email or password")
	}
	return id, nil
}

// ResetPassword sets a new password and revokes every session of the user.
func (s *Service) ResetPassword(ctx context.Context, email, password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	return s.tx(ctx, func(tx *sql.Tx) error {
		var id int64
		if err := tx.QueryRowContext(ctx, "UPDATE users SET password_hash = ? WHERE email = ? RETURNING id", hash, email).Scan(&id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errf(CodeNotFound, "no user with email %q", email)
			}
			return err
		}
		_, err := tx.ExecContext(ctx, "UPDATE refresh_tokens SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL", ts(s.now()), id)
		return err
	})
}

// IssueRefresh creates a refresh token for userID.
func (s *Service) IssueRefresh(ctx context.Context, userID int64) (string, error) {
	return s.issueRefresh(ctx, s.db, userID)
}

func (s *Service) issueRefresh(ctx context.Context, q querier, userID int64) (string, error) {
	token, hash := auth.NewOpaque("")
	_, err := q.ExecContext(ctx, "INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES (?, ?, ?)",
		userID, hash, ts(s.now().Add(RefreshTTL)))
	return token, err
}

// RotateRefresh consumes a refresh token and returns its user and a new
// token. Presenting an already-used token revokes all of the user's sessions,
// since it indicates the token was stolen.
func (s *Service) RotateRefresh(ctx context.Context, token string) (int64, string, error) {
	var (
		userID  int64
		newTok  string
		revoked bool
	)
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var (
			id      int64
			expires string
			rev     sql.NullString
		)
		err := tx.QueryRowContext(ctx, "SELECT id, user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = ?",
			auth.HashOpaque(token)).Scan(&id, &userID, &expires, &rev)
		if errors.Is(err, sql.ErrNoRows) {
			return errf(CodeAuth, "invalid refresh token")
		}
		if err != nil {
			return err
		}
		now := ts(s.now())
		if rev.Valid {
			revoked = true
			_, err := tx.ExecContext(ctx, "UPDATE refresh_tokens SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL", now, userID)
			return err
		}
		if expires <= now {
			return errf(CodeAuth, "refresh token expired")
		}
		if _, err := tx.ExecContext(ctx, "UPDATE refresh_tokens SET revoked_at = ? WHERE id = ?", now, id); err != nil {
			return err
		}
		newTok, err = s.issueRefresh(ctx, tx, userID)
		return err
	})
	if err != nil {
		return 0, "", err
	}
	if revoked {
		return 0, "", errf(CodeAuth, "refresh token reuse detected; all sessions were revoked")
	}
	return userID, newTok, nil
}

// RevokeRefresh revokes a refresh token (logout).
func (s *Service) RevokeRefresh(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE refresh_tokens SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL",
		ts(s.now()), auth.HashOpaque(token))
	return err
}

// AccessToken is a personal access token as listed to its owner.
type AccessToken struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	// Token is only set in the creation response.
	Token string `json:"token,omitempty"`
}

// CreateAccessToken creates a PAT. The plain token is returned only once.
func (s *Service) CreateAccessToken(ctx context.Context, userID int64, name string) (*AccessToken, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return nil, errf(CodeInvalid, "token name must be 1-100 characters")
	}
	token, hash := auth.NewOpaque(auth.PATPrefix)
	at := &AccessToken{Name: name, Prefix: token[:len(auth.PATPrefix)+6], CreatedAt: s.now(), Token: token}
	err := s.db.QueryRowContext(ctx,
		"INSERT INTO access_tokens (user_id, name, token_hash, prefix, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id",
		userID, name, hash, at.Prefix, ts(at.CreatedAt)).Scan(&at.ID)
	return at, err
}

// ListAccessTokens lists the active PATs of a user.
func (s *Service) ListAccessTokens(ctx context.Context, userID int64) ([]AccessToken, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, prefix, created_at, last_used_at FROM access_tokens WHERE user_id = ? AND revoked_at IS NULL ORDER BY id", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AccessToken{}
	for rows.Next() {
		var (
			at      AccessToken
			created string
			used    sql.NullString
		)
		if err := rows.Scan(&at.ID, &at.Name, &at.Prefix, &created, &used); err != nil {
			return nil, err
		}
		at.CreatedAt = parseTS(created)
		at.LastUsedAt = nullTime(used)
		out = append(out, at)
	}
	return out, rows.Err()
}

// RevokeAccessToken revokes a PAT.
func (s *Service) RevokeAccessToken(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, "UPDATE access_tokens SET revoked_at = ? WHERE id = ? AND user_id = ? AND revoked_at IS NULL",
		ts(s.now()), id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// LookupAccessToken returns the owner of an active PAT.
func (s *Service) LookupAccessToken(ctx context.Context, token string) (int64, error) {
	var id, userID int64
	err := s.db.QueryRowContext(ctx, "SELECT id, user_id FROM access_tokens WHERE token_hash = ? AND revoked_at IS NULL",
		auth.HashOpaque(token)).Scan(&id, &userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errf(CodeAuth, "invalid access token")
	}
	if err != nil {
		return 0, err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE access_tokens SET last_used_at = ? WHERE id = ?", ts(s.now()), id)
	return userID, err
}

// Me returns the profile of userID.
func (s *Service) Me(ctx context.Context, userID int64) (*User, error) {
	var (
		u       User
		created string
	)
	err := s.db.QueryRowContext(ctx, "SELECT id, email, timezone, created_at FROM users WHERE id = ?", userID).
		Scan(&u.ID, &u.Email, &u.Timezone, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	u.CreatedAt = parseTS(created)
	return &u, err
}

// UpdateTimezone changes a user's timezone.
func (s *Service) UpdateTimezone(ctx context.Context, userID int64, tz string) (*User, error) {
	if err := validateTimezone(tz); err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE users SET timezone = ? WHERE id = ?", tz, userID); err != nil {
		return nil, err
	}
	s.publish(userID, "me", "tasks")
	return s.Me(ctx, userID)
}

// WorkingDay is the working hours of one weekday (0 = Sunday).
type WorkingDay struct {
	Weekday int `json:"weekday"`
	dates.Hours
}

// WorkingHours returns the working days of a user.
func (s *Service) WorkingHours(ctx context.Context, userID int64) ([]WorkingDay, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT weekday, start_min, end_min FROM working_hours WHERE user_id = ? ORDER BY weekday", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorkingDay{}
	for rows.Next() {
		var d WorkingDay
		if err := rows.Scan(&d.Weekday, &d.Start, &d.End); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SetWorkingHours replaces the working days of a user. Weekdays not listed
// are days off.
func (s *Service) SetWorkingHours(ctx context.Context, userID int64, days []WorkingDay) ([]WorkingDay, error) {
	seen := map[int]bool{}
	for _, d := range days {
		if d.Weekday < 0 || d.Weekday > 6 || seen[d.Weekday] {
			return nil, errf(CodeInvalid, "invalid or duplicate weekday %d", d.Weekday)
		}
		seen[d.Weekday] = true
		if d.Start < 0 || d.End > 1440 || d.Start >= d.End {
			return nil, errf(CodeInvalid, "invalid hours for weekday %d", d.Weekday)
		}
	}
	err := s.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM working_hours WHERE user_id = ?", userID); err != nil {
			return err
		}
		for _, d := range days {
			if _, err := tx.ExecContext(ctx, "INSERT INTO working_hours (user_id, weekday, start_min, end_min) VALUES (?, ?, ?, ?)",
				userID, d.Weekday, d.Start, d.End); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.publish(userID, "me")
	return s.WorkingHours(ctx, userID)
}
