package service

import (
	"context"

	"github.com/masterfabric-go/masterfabric/internal/domain/execution/model"
)

// Runner is the only boundary through which code may be submitted for
// execution. Implementations must run outside the API process and return
// signed provenance; the API never interprets source as an OS command.
type Runner interface {
	Execute(ctx context.Context, request model.Request) (model.Result, error)
}
