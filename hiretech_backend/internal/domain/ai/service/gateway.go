package service

import (
	"context"

	aiModel "github.com/masterfabric-go/masterfabric/internal/domain/ai/model"
)

type Gateway interface {
	Complete(ctx context.Context, request aiModel.CompletionRequest) (aiModel.CompletionResponse, error)
}
