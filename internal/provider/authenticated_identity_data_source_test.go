package provider

import (
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/stretchr/testify/assert"
)

func TestMapAuthenticatedIdentityToDataSourceModel(t *testing.T) {
	got := mapAuthenticatedIdentityToDataSourceModel(&v1.GetAuthenticatedIdentityResponse{
		OrganizationId:   "org-123",
		OrganizationTier: "pro",
		Subject: &v1.Subject{
			Id:        "subject-123",
			Principal: v1.Principal_PRINCIPAL_USER,
		},
	})

	assert.Equal(t, "subject-123", got.ID.ValueString())
	assert.Equal(t, v1.Principal_PRINCIPAL_USER.String(), got.Principal.ValueString())
	assert.Equal(t, "org-123", got.OrganizationID.ValueString())
	assert.Equal(t, "pro", got.OrganizationTier.ValueString())
}
