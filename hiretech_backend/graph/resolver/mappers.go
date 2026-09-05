package resolver

import (
	"strings"

	"github.com/masterfabric-go/masterfabric/graph/model"
	"github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	tenantModel "github.com/masterfabric-go/masterfabric/internal/domain/tenant/model"
)

func organizationModel(org *tenantModel.Organization) *model.Organization {
	return &model.Organization{
		ID: org.ID, Name: org.Name, Slug: org.Slug,
		Status: model.OrganizationStatus(strings.ToUpper(string(org.Status))), CreatedAt: org.CreatedAt,
	}
}

func membershipModel(selected *usecase.OrganizationMembership) *model.OrganizationMembership {
	return &model.OrganizationMembership{
		Organization: organizationModel(selected.Organization),
		Status:       model.MembershipStatus(strings.ToUpper(string(selected.Membership.Status))),
	}
}
