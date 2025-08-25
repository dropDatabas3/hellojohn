package mysql

import (
	"context"
	"fmt"

	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type Store struct{}

func New(ctx context.Context, dsn string) (core.Repository, error) {
	return nil, fmt.Errorf("mysql: %w", core.ErrNotImplemented)
}
