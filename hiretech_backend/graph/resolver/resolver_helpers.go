package resolver

import (
	"github.com/masterfabric-go/masterfabric/graph/generated"
	"github.com/masterfabric-go/masterfabric/graph/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

func notConfigured(name string) error {
	return domainErr.New(domainErr.ErrNotImplemented, name+" is not configured", nil)
}

func stringOrEmpty(value *model.InterviewLanguage) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func stringOrEmptyQuestionSource(value *model.QuestionSource) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

var _ generated.InterviewResolver = (*interviewResolver)(nil)
var _ generated.MutationResolver = (*mutationResolver)(nil)
var _ generated.QueryResolver = (*queryResolver)(nil)
