package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"certificate-inspector/internal/job"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Migrate(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(files)

	for _, file := range files {
		contents, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := s.db.Exec(string(contents)); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) CreateJob(ctx context.Context, candidate job.Job) (job.Job, error) {
	now := time.Now().UTC()
	candidate.CreatedAt = now
	candidate.UpdatedAt = now
	if candidate.Status == "" {
		candidate.Status = job.StatusPending
	}
	if candidate.MaxAttempts == 0 {
		candidate.MaxAttempts = 2
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO jobs (
	id, hostname, port, status, attempts, max_attempts, error, result_json,
	idempotency_key, created_at, updated_at, started_at, finished_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		candidate.ID,
		candidate.Hostname,
		candidate.Port,
		string(candidate.Status),
		candidate.Attempts,
		candidate.MaxAttempts,
		nullableString(candidate.Error),
		nullableString(candidate.ResultJSON),
		nullableString(candidate.IdempotencyKey),
		formatTime(candidate.CreatedAt),
		formatTime(candidate.UpdatedAt),
		formatTimePtr(candidate.StartedAt),
		formatTimePtr(candidate.FinishedAt),
	)
	return candidate, err
}

func (s *SQLiteStore) GetJob(ctx context.Context, id string) (job.Job, error) {
	return s.queryJob(ctx, `SELECT * FROM jobs WHERE id = ?`, id)
}

func (s *SQLiteStore) GetJobByIdempotencyKey(ctx context.Context, key string) (job.Job, error) {
	return s.queryJob(ctx, `SELECT * FROM jobs WHERE idempotency_key = ?`, key)
}

func (s *SQLiteStore) ListRecentJobs(ctx context.Context, limit int) ([]job.Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	rows, err := s.db.QueryContext(ctx, `SELECT * FROM jobs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []job.Job{}
	for rows.Next() {
		item, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, item)
	}
	return jobs, rows.Err()
}

func (s *SQLiteStore) MarkRunning(ctx context.Context, id string) (job.Job, error) {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
UPDATE jobs SET status = ?, attempts = attempts + 1, updated_at = ?, started_at = COALESCE(started_at, ?) WHERE id = ?`,
		string(job.StatusRunning), formatTime(now), formatTime(now), id)
	if err != nil {
		return job.Job{}, err
	}
	return s.GetJob(ctx, id)
}

func (s *SQLiteStore) MarkSucceeded(ctx context.Context, id string, resultJSON string) (job.Job, error) {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
UPDATE jobs SET status = ?, result_json = ?, error = NULL, updated_at = ?, finished_at = ? WHERE id = ?`,
		string(job.StatusSucceeded), resultJSON, formatTime(now), formatTime(now), id)
	if err != nil {
		return job.Job{}, err
	}
	return s.GetJob(ctx, id)
}

func (s *SQLiteStore) MarkFailed(ctx context.Context, id string, message string) (job.Job, error) {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
UPDATE jobs SET status = ?, error = ?, updated_at = ?, finished_at = ? WHERE id = ?`,
		string(job.StatusFailed), message, formatTime(now), formatTime(now), id)
	if err != nil {
		return job.Job{}, err
	}
	return s.GetJob(ctx, id)
}

func (s *SQLiteStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) queryJob(ctx context.Context, query string, args ...any) (job.Job, error) {
	row := s.db.QueryRowContext(ctx, query, args...)
	item, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return job.Job{}, ErrNotFound
	}
	return item, err
}

type jobScanner interface {
	Scan(dest ...any) error
}

func scanJob(scanner jobScanner) (job.Job, error) {
	var item job.Job
	var status string
	var errValue, resultValue, keyValue sql.NullString
	var startedValue, finishedValue sql.NullString
	var createdValue, updatedValue string

	err := scanner.Scan(
		&item.ID,
		&item.Hostname,
		&item.Port,
		&status,
		&item.Attempts,
		&item.MaxAttempts,
		&errValue,
		&resultValue,
		&keyValue,
		&createdValue,
		&updatedValue,
		&startedValue,
		&finishedValue,
	)
	if err != nil {
		return job.Job{}, err
	}

	item.Status = job.Status(status)
	item.Error = errValue.String
	item.ResultJSON = resultValue.String
	item.IdempotencyKey = keyValue.String
	item.CreatedAt = parseTime(createdValue)
	item.UpdatedAt = parseTime(updatedValue)
	item.StartedAt = parseTimePtr(startedValue)
	item.FinishedAt = parseTimePtr(finishedValue)
	return item, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func formatTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func parseTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}

func parseTimePtr(value sql.NullString) *time.Time {
	if !value.Valid {
		return nil
	}
	parsed := parseTime(value.String)
	return &parsed
}
