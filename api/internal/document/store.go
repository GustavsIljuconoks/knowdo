package document

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("document not found")

// Store is the persistence boundary for documents. Handlers depend on
// this interface, not on *PGStore directly, so tests can swap in a fake.
type Store interface {
	Create(ctx context.Context, d *Document, content []byte) error
	List(ctx context.Context) ([]Document, error)
	Get(ctx context.Context, id int64) (Document, error)
}

type PGStore struct {
	pool *pgxpool.Pool
}

func NewPGStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool}
}

// Create inserts the metadata and the file bytes together, filling in
// the DB-generated fields from the RETURNING clause.
func (s *PGStore) Create(ctx context.Context, d *Document, content []byte) error {
	const q = `
		INSERT INTO documents (filename, content_type, content)
		VALUES ($1, $2, $3)
		RETURNING id, status, chunk_count, created_at`

	return s.pool.QueryRow(ctx, q, d.Filename, d.ContentType, content).
		Scan(&d.ID, &d.Status, &d.ChunkCount, &d.CreatedAt)
}

// List never selects content — a list endpoint that loads every
// uploaded file into memory is a trivially avoidable failure.
func (s *PGStore) List(ctx context.Context) ([]Document, error) {
	const q = `
		SELECT id, filename, content_type, status, error, chunk_count, created_at
		FROM documents ORDER BY id`

	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	docs := []Document{}
	for rows.Next() {
		var d Document

		if err := rows.Scan(&d.ID, &d.Filename, &d.ContentType, &d.Status, &d.Error, &d.ChunkCount, &d.CreatedAt); err != nil {
			return nil, err
		}

		docs = append(docs, d)
	}

	return docs, rows.Err()
}

func (s *PGStore) Get(ctx context.Context, id int64) (Document, error) {
	const q = `
		SELECT id, filename, content_type, status, error, chunk_count, created_at
		FROM documents WHERE id = $1`

	var d Document

	err := s.pool.QueryRow(ctx, q, id).
		Scan(&d.ID, &d.Filename, &d.ContentType, &d.Status, &d.Error, &d.ChunkCount, &d.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, ErrNotFound
	}

	return d, err
}
