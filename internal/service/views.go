package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"kala/internal/filter"
)

// View is a saved filter.
type View struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Query    string `json:"query"`
	Position int    `json:"position"`
	Builtin  bool   `json:"builtin"`
}

// Views lists the saved views of a user.
func (s *Service) Views(ctx context.Context, userID int64) ([]View, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, query, position, builtin FROM views WHERE user_id = ? ORDER BY position, id", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []View{}
	for rows.Next() {
		var v View
		if err := rows.Scan(&v.ID, &v.Name, &v.Query, &v.Position, &v.Builtin); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ViewInput creates or updates a view.
type ViewInput struct {
	Name     Optional[string] `json:"name"`
	Query    Optional[string] `json:"query"`
	Position Optional[int]    `json:"position"`
}

func (s *Service) checkView(ctx context.Context, userID int64, in ViewInput) error {
	if in.Name.Set && (strings.TrimSpace(in.Name.Value) == "" || len(in.Name.Value) > 100) {
		return errf(CodeInvalid, "view name must be 1-100 characters")
	}
	if in.Query.Set {
		env, err := s.Env(ctx, userID)
		if err != nil {
			return err
		}
		if _, err := filter.Compile(in.Query.Value, env); err != nil {
			return errf(CodeInvalid, "filter: %s", err.Error())
		}
	}
	return nil
}

func uniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// CreateView saves a new view at the end of the list.
func (s *Service) CreateView(ctx context.Context, userID int64, in ViewInput) (*View, error) {
	if !in.Name.Set || !in.Query.Set {
		return nil, errf(CodeInvalid, "name and query are required")
	}
	if err := s.checkView(ctx, userID, in); err != nil {
		return nil, err
	}
	var id int64
	err := s.db.QueryRowContext(ctx, `INSERT INTO views (user_id, name, query, position)
		VALUES (?, ?, ?, (SELECT COALESCE(MAX(position), -1) + 1 FROM views WHERE user_id = ?)) RETURNING id`,
		userID, strings.TrimSpace(in.Name.Value), in.Query.Value, userID).Scan(&id)
	if uniqueViolation(err) {
		return nil, errf(CodeConflict, "a view named %q already exists", in.Name.Value)
	}
	if err != nil {
		return nil, err
	}
	s.publish(userID, "views")
	return s.view(ctx, userID, id)
}

func (s *Service) view(ctx context.Context, userID, id int64) (*View, error) {
	var v View
	err := s.db.QueryRowContext(ctx, "SELECT id, name, query, position, builtin FROM views WHERE id = ? AND user_id = ?", id, userID).
		Scan(&v.ID, &v.Name, &v.Query, &v.Position, &v.Builtin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &v, err
}

// UpdateView edits a view.
func (s *Service) UpdateView(ctx context.Context, userID, id int64, in ViewInput) (*View, error) {
	if err := s.checkView(ctx, userID, in); err != nil {
		return nil, err
	}
	v, err := s.view(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if in.Name.Set {
		v.Name = strings.TrimSpace(in.Name.Value)
	}
	if in.Query.Set {
		v.Query = in.Query.Value
	}
	if in.Position.Set {
		v.Position = in.Position.Value
	}
	_, err = s.db.ExecContext(ctx, "UPDATE views SET name = ?, query = ?, position = ? WHERE id = ?", v.Name, v.Query, v.Position, id)
	if uniqueViolation(err) {
		return nil, errf(CodeConflict, "a view named %q already exists", v.Name)
	}
	if err != nil {
		return nil, err
	}
	s.publish(userID, "views")
	return v, nil
}

// DeleteView deletes a view.
func (s *Service) DeleteView(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM views WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	s.publish(userID, "views")
	return nil
}
