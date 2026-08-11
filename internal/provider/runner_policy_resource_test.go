package provider

import (
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapRunnerPolicyToModel(t *testing.T) {
	// The API returns only the group and role, so the runner ID and the
	// composite resource ID have to come from the caller.
	got := mapRunnerPolicyToModel("runner-1", &v1.RunnerPolicy{
		GroupId: "group-1",
		Role:    v1.RunnerRole_RUNNER_ROLE_ADMIN,
	})

	assert.Equal(t, "runner-1/group-1", got.ID.ValueString())
	assert.Equal(t, "runner-1", got.RunnerID.ValueString())
	assert.Equal(t, "group-1", got.GroupID.ValueString())
	assert.Equal(t, "RUNNER_ROLE_ADMIN", got.Role.ValueString())
}

func TestRunnerRoleValue(t *testing.T) {
	t.Run("accepts a documented role", func(t *testing.T) {
		var diags diag.Diagnostics
		got := runnerRoleValue(types.StringValue("RUNNER_ROLE_USER"), &diags)
		require.False(t, diags.HasError())
		assert.Equal(t, v1.RunnerRole_RUNNER_ROLE_USER, got)
	})

	t.Run("rejects an unknown role instead of sending unspecified", func(t *testing.T) {
		var diags diag.Diagnostics
		runnerRoleValue(types.StringValue("admin"), &diags)
		require.True(t, diags.HasError())
		assert.Contains(t, diags.Errors()[0].Detail(), "RUNNER_ROLE_ADMIN")
	})
}
