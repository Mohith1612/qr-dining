package testutil

import (
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// NewTestRepos returns a *repository.Repos backed by the test pool with a nop logger.
func NewTestRepos(pool *pgxpool.Pool) *repository.Repos {
	return repository.New(pool, zerolog.Nop())
}
