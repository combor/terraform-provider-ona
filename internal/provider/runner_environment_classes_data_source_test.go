package provider

import (
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapRunnerEnvironmentClassesToDataSourceModel_MapsAndSortsClasses(t *testing.T) {
	got, diags := mapRunnerEnvironmentClassesToDataSourceModel("runner-123", []*v1.EnvironmentClass{
		{
			Id:          "env-b",
			DisplayName: "Regular",
			Description: "4 vCPU / 16 GiB",
			Configuration: []*v1.FieldValue{
				{Key: "instanceType", Value: "m6i.xlarge"},
				{Key: "vcpus", Value: "4"},
			},
			RunnerId: "runner-123",
			Enabled:  true,
		},
		{
			Id:          "env-a",
			DisplayName: "Small",
			Description: "2 vCPU / 8 GiB",
			Configuration: []*v1.FieldValue{
				{Key: "instanceType", Value: "m6i.large"},
				{Key: "vcpus", Value: "2"},
			},
			RunnerId: "runner-123",
			Enabled:  false,
		},
	})

	require.False(t, diags.HasError())
	assert.Equal(t, "runner-123", got.RunnerID.ValueString())
	require.Len(t, got.EnvironmentClasses, 2)

	assert.Equal(t, "env-a", got.EnvironmentClasses[0].ID.ValueString())
	assert.Equal(t, "Small", got.EnvironmentClasses[0].DisplayName.ValueString())
	assert.Equal(t, "2 vCPU / 8 GiB", got.EnvironmentClasses[0].Description.ValueString())
	assert.Equal(t, "runner-123", got.EnvironmentClasses[0].RunnerID.ValueString())
	assert.False(t, got.EnvironmentClasses[0].Enabled.ValueBool())

	config := got.EnvironmentClasses[0].Configuration.Elements()
	instanceType, ok := config["instanceType"].(types.String)
	require.True(t, ok)
	assert.Equal(t, "m6i.large", instanceType.ValueString())
	vcpus, ok := config["vcpus"].(types.String)
	require.True(t, ok)
	assert.Equal(t, "2", vcpus.ValueString())

	assert.Equal(t, "env-b", got.EnvironmentClasses[1].ID.ValueString())
	assert.True(t, got.EnvironmentClasses[1].Enabled.ValueBool())
}

func TestMapRunnerEnvironmentClassesToDataSourceModel_EmptyList(t *testing.T) {
	got, diags := mapRunnerEnvironmentClassesToDataSourceModel("runner-123", []*v1.EnvironmentClass{})
	require.False(t, diags.HasError())
	assert.Equal(t, "runner-123", got.RunnerID.ValueString())
	assert.Empty(t, got.EnvironmentClasses)
}

func TestMapRunnerEnvironmentClassesToDataSourceModel_EmptyOptionalFields(t *testing.T) {
	got, diags := mapRunnerEnvironmentClassesToDataSourceModel("runner-123", []*v1.EnvironmentClass{
		{
			Id:       "env-1",
			RunnerId: "runner-123",
			Enabled:  true,
		},
	})
	require.False(t, diags.HasError())
	require.Len(t, got.EnvironmentClasses, 1)
	assert.True(t, got.EnvironmentClasses[0].DisplayName.IsNull())
	assert.True(t, got.EnvironmentClasses[0].Description.IsNull())
	assert.Empty(t, got.EnvironmentClasses[0].Configuration.Elements())
}
