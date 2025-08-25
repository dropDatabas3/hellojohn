package mongo

import (
	"context"
	"fmt"

	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type Store struct{}

func New(ctx context.Context, uri, database string) (core.Repository, error) {
	return nil, fmt.Errorf("mongo: %w", core.ErrNotImplemented)
}
