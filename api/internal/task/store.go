package task

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("task not found")

// Store is the persistence boundary for tasks. Handlers depend on this
// interface, not on *PGStore directly, so tests can swap in a fake.
type Store interface {
	Create(ctx context.Context, t *Task) error
	List(ctx context.Context) ([]Task, error)
	Get(ctx context.Context, id int64) (Task, error)
	Update(ctx context.Context, id int64, patch Patch) (Task, error)
	Delete(ctx context.Context, id int64) error
}

// Patch holds the fields a PATCH request may update. A nil field means
// "leave as-is" — that's what lets one request update just the status,
// say, without also having to resend title/description/due_date.
type Patch struct {
	Title       *string    `json:"title"`
	Description *string    `json:"description"`
	Status      *string    `json:"status"`
	DueDate     *time.Time `json:"due_date"`
}

type PGStore struct {
	pool *pgxpool.Pool
}

func NewPGStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool}
}

func (s *PGStore) List(ctx context.Context) ([]Task, error) {
	const q = `SELECT id, title, description, status, due_date, created_at FROM tasks ORDER BY id`

	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var t Task

		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.DueDate, &t.CreatedAt); err != nil {
			return nil, err
		}

		tasks = append(tasks, t)
	}

	return tasks, rows.Err()
}

// Create inserts t and fills in its DB-generated fields (id, status
// default, created_at) from the RETURNING clause.
func (s *PGStore) Create(ctx context.Context, t *Task) error {
	const q = `
		INSERT INTO tasks (title, description, due_date)
		VALUES ($1, $2, $3)
		RETURNING id, status, created_at`

	return s.pool.QueryRow(ctx, q, t.Title, t.Description, t.DueDate).
		Scan(&t.ID, &t.Status, &t.CreatedAt)
}

func (s *PGStore) Get(ctx context.Context, id int64) (Task, error) {
	const q = `SELECT id, title, description, status, due_date, created_at FROM tasks WHERE id = $1`
	var t Task

	err := s.pool.QueryRow(ctx, q, id).
		Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.DueDate, &t.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}

	return t, err
}

// Update applies patch to the task with the given id. COALESCE keeps
// each column's current value when the matching patch field is nil, so
// one query handles "update just one field" and "update all of them"
// the same way.
func (s *PGStore) Update(ctx context.Context, id int64, patch Patch) (Task, error) {
	const q = `
		UPDATE tasks
		SET
			title       = COALESCE($2, title),
			description = COALESCE($3, description),
			status      = COALESCE($4, status),
			due_date    = COALESCE($5, due_date)
		WHERE id = $1
		RETURNING id, title, description, status, due_date, created_at`

	var t Task

	err := s.pool.QueryRow(ctx, q, id, patch.Title, patch.Description, patch.Status, patch.DueDate).
		Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.DueDate, &t.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}

	return t, err
}

func (s *PGStore) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM tasks WHERE id = $1`

	tag, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
