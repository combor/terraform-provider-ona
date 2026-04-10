package provider

import (
	"testing"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/gitpod-io/gitpod-sdk-go/shared"
	"github.com/stretchr/testify/assert"
)

func TestMapAuthenticatedIdentityToDataSourceModel(t *testing.T) {
	got := mapAuthenticatedIdentityToDataSourceModel(&gitpod.IdentityGetAuthenticatedIdentityResponse{
		OrganizationID:   "org-123",
		OrganizationTier: "pro",
		Subject: shared.Subject{
			ID:        "subject-123",
			Principal: shared.PrincipalUser,
		},
	})

	assert.Equal(t, "subject-123", got.ID.ValueString())
	assert.Equal(t, string(shared.PrincipalUser), got.Principal.ValueString())
	assert.Equal(t, "org-123", got.OrganizationID.ValueString())
	assert.Equal(t, "pro", got.OrganizationTier.ValueString())
}
