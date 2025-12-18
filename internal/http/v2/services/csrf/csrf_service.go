package csrf

import "context"

type Service interface {
	GetToken(ctx context.Context) (string, error)
}
