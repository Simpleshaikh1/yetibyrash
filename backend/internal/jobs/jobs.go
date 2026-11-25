// Package jobs runs background work (sending emails) from a table in Postgres.
// Work is added in the same database transaction as the change that causes it,
// so an email is never sent for an order change that was rolled back, and never lost
// if the server stops right after the change.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"yetibyrash/backend/internal/db/sqlc"
)

// MaxAttempts is how often a job is tried before it's left for staff to look at.
const MaxAttempts = 8

// Kinds of job.
const KindOrderEmail = "order_email"

// OrderEmail asks for the customer message for an order step (e.g. "shipped").
type OrderEmail struct {
	OrderID int64  `json:"orderId"`
	Event   string `json:"event"`
	Note    string `json:"note"` // staff's note to the customer, "" if none
}

// Enqueue adds a job using q, which may be inside a transaction.
func Enqueue(ctx context.Context, q *sqlc.Queries, kind string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return q.EnqueueJob(ctx, sqlc.EnqueueJobParams{Kind: kind, Payload: b})
}

// Handler does one job. Returning an error makes it retry later.
type Handler func(ctx context.Context, payload []byte) error

type Runner struct {
	pool     *pgxpool.Pool
	q        *sqlc.Queries
	handlers map[string]Handler
}

func NewRunner(pool *pgxpool.Pool) *Runner {
	return &Runner{pool: pool, q: sqlc.New(pool), handlers: map[string]Handler{}}
}

func (r *Runner) Handle(kind string, h Handler) { r.handlers[kind] = h }

// RunOnce does every job that's due and returns how many it finished.
func (r *Runner) RunOnce(ctx context.Context) (int, error) {
	done := 0
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		due, err := q.ClaimDueJobs(ctx, MaxAttempts)
		if err != nil {
			return err
		}
		for _, j := range due {
			h, ok := r.handlers[j.Kind]
			if !ok {
				err = fmt.Errorf("no handler for job kind %q", j.Kind)
			} else {
				err = h(ctx, j.Payload)
			}
			if err == nil {
				if err := q.MarkJobDone(ctx, j.ID); err != nil {
					return err
				}
				done++
				continue
			}
			retry := backoff(int(j.Attempts))
			slog.Warn("job failed", "id", j.ID, "kind", j.Kind, "attempt", j.Attempts+1, "retry_in", retry, "error", err)
			if err := q.MarkJobFailed(ctx, sqlc.MarkJobFailedParams{ID: j.ID, LastError: truncate(err.Error(), 500), RetryInSeconds: int32(retry.Seconds())}); err != nil {
				return err
			}
		}
		return nil
	})
	return done, err
}

// Run works through jobs every few seconds until ctx ends.
func (r *Runner) Run(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := r.RunOnce(ctx); err != nil && ctx.Err() == nil {
				slog.Error("run jobs", "error", err)
			}
		}
	}
}

// backoff waits 30s, 1m, 2m, 4m… up to an hour between tries.
func backoff(attempts int) time.Duration {
	d := 30 * time.Second << attempts
	if d > time.Hour || d <= 0 {
		return time.Hour
	}
	return d
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
