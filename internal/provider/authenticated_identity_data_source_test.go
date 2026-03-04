package provider

import (
	"testing"

	"github.com/gitpod-io/gitpod-sdk-go/shared"
	"github.com/stretchr/testify/assert"
)

func TestMapAuthenticatedIdentityToDataSourceModel(t *testing.T) {
	got := mapAuthenticatedIdentityToDataSourceModel("org-123", shared.Subject{
		ID:        "subject-123",
		Principal: shared.PrincipalUser,
	})

	assert.Equal(t, "subject-123", got.ID.ValueString())
	assert.Equal(t, string(shared.PrincipalUser), got.Principal.ValueString())
	assert.Equal(t, "org-123", got.OrganizationID.ValueString())
}
