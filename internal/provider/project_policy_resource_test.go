package provider

import (
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapProjectPolicyToModel(t *testing.T) {
	got := mapProjectPolicyToModel("project-1", &v1.ProjectPolicy{
		GroupId: "group-1",
		Role:    v1.ProjectRole_PROJECT_ROLE_EDITOR,
	})

	assert.Equal(t, "project-1/group-1", got.ID.ValueString())
	assert.Equal(t, "project-1", got.ProjectID.ValueString())
	assert.Equal(t, "group-1", got.GroupID.ValueString())
	assert.Equal(t, "PROJECT_ROLE_EDITOR", got.Role.ValueString())
}

func TestProjectRoleValue(t *testing.T) {
	t.Run("accepts every documented role", func(t *testing.T) {
		for name, want := range map[string]v1.ProjectRole{
			"PROJECT_ROLE_ADMIN":  v1.ProjectRole_PROJECT_ROLE_ADMIN,
			"PROJECT_ROLE_EDITOR": v1.ProjectRole_PROJECT_ROLE_EDITOR,
			"PROJECT_ROLE_USER":   v1.ProjectRole_PROJECT_ROLE_USER,
		} {
			var diags diag.Diagnostics
			got := projectRoleValue(types.StringValue(name), &diags)
			require.False(t, diags.HasError(), name)
			assert.Equal(t, want, got)
		}
	})

	t.Run("rejects a runner role used by mistake", func(t *testing.T) {
		var diags diag.Diagnostics
		projectRoleValue(types.StringValue("RUNNER_ROLE_ADMIN"), &diags)
		require.True(t, diags.HasError())
		assert.Contains(t, diags.Errors()[0].Detail(), "PROJECT_ROLE_ADMIN")
	})
}
