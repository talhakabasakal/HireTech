package resolver

import (
	aiadminUsecase "github.com/masterfabric-go/masterfabric/internal/application/aiadmin/usecase"
	evaluationUsecase "github.com/masterfabric-go/masterfabric/internal/application/evaluation/usecase"
	iamUsecase "github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	interviewUsecase "github.com/masterfabric-go/masterfabric/internal/application/interview/usecase"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	UseCase              *iamUsecase.OrganizationContextUseCase
	InterviewUseCase     *interviewUsecase.InterviewService
	EvaluationUseCase    *evaluationUsecase.EvaluationService
	QuestionDraftUseCase *interviewUsecase.QuestionDraftService
	AIAdminUseCase       *aiadminUsecase.Service
}
