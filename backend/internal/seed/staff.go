package seed

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"yetibyrash/backend/internal/staff"
)

// DevOwner creates an owner account for local development if there are no staff yet.
// It reports whether it created one.
func DevOwner(ctx context.Context, pool *pgxpool.Pool, email, password string) (bool, error) {
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM staff`).Scan(&count); err != nil {
		return false, fmt.Errorf("count staff: %w", err)
	}
	if count > 0 {
		return false, nil
	}
	if _, err := staff.NewService(pool).Create(ctx, email, "Dev Owner", staff.RoleOwner, password); err != nil {
		return false, fmt.Errorf("create dev owner: %w", err)
	}
	return true, nil
}
