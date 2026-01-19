// Package mysql implementa el adapter MySQL (placeholder).
package mysql

import (
	"context"
	"fmt"

	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

func init() {
	store.RegisterAdapter(&mysqlAdapter{})
}

type mysqlAdapter struct{}

func (a *mysqlAdapter) Name() string { return "mysql" }

func (a *mysqlAdapter) Connect(ctx context.Context, cfg store.AdapterConfig) (store.AdapterConnection, error) {
	// TODO: Implementar usando database/sql con github.com/go-sql-driver/mysql
	return nil, fmt.Errorf("mysql adapter not yet implemented")
}
