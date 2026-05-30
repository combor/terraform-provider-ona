package provider

import (
	"testing"

	"github.com/gitpod-io/gitpod-sdk-go/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapEnvironmentClassToModel_AWSEC2Configuration(t *testing.T) {
	environmentClass := shared.EnvironmentClass{
		ID:          "env-class-aws-ec2",
		RunnerID:    "runner-aws-ec2",
		DisplayName: "Large",
		Description: "8 vCPU / 32 GiB / 200 GiB disk",
		Configuration: []shared.FieldValue{
			{Key: "instanceType", Value: "m6i.2xlarge"},
			{Key: "diskSizeGB", Value: "200"},
			{Key: "spot", Value: "false"},
		},
		Enabled: true,
	}

	got := mapEnvironmentClassToModel(environmentClass)

	assert.Equal(t, "env-class-aws-ec2", got.ID.ValueString())
	assert.Equal(t, "runner-aws-ec2", got.RunnerID.ValueString())
	assert.Equal(t, "Large", got.DisplayName.ValueString())
	assert.Equal(t, "8 vCPU / 32 GiB / 200 GiB disk", got.Description.ValueString())
	assert.True(t, got.Enabled.ValueBool())

	require.NotNil(t, got.Configuration)
	assert.Equal(t, "m6i.2xlarge", got.Configuration.InstanceType.ValueString())
	assert.Equal(t, int64(200), got.Configuration.DiskSizeGB.ValueInt64())
	assert.False(t, got.Configuration.Spot.ValueBool())
}

func TestMapEnvironmentClassToModel_EmptyOptionalFields(t *testing.T) {
	environmentClass := shared.EnvironmentClass{
		ID:            "env-class-789",
		RunnerID:      "runner-abc",
		DisplayName:   "",
		Description:   "",
		Configuration: []shared.FieldValue{},
		Enabled:       false,
	}

	got := mapEnvironmentClassToModel(environmentClass)

	assert.Equal(t, "env-class-789", got.ID.ValueString())
	assert.Equal(t, "runner-abc", got.RunnerID.ValueString())
	assert.True(t, got.DisplayName.IsNull())
	assert.True(t, got.Description.IsNull())
	assert.False(t, got.Enabled.ValueBool())
	assert.Nil(t, got.Configuration)
}

func TestMapEnvironmentClassToModel_MinimalConfiguration(t *testing.T) {
	environmentClass := shared.EnvironmentClass{
		ID:       "env-class-minimal",
		RunnerID: "runner-aws-ec2",
		Configuration: []shared.FieldValue{
			{Key: "instanceType", Value: "t3.medium"},
		},
		Enabled: true,
	}

	got := mapEnvironmentClassToModel(environmentClass)

	assert.Equal(t, "env-class-minimal", got.ID.ValueString())
	assert.Equal(t, "runner-aws-ec2", got.RunnerID.ValueString())

	require.NotNil(t, got.Configuration)
	assert.Equal(t, "t3.medium", got.Configuration.InstanceType.ValueString())
	assert.True(t, got.Configuration.DiskSizeGB.IsNull())
	assert.True(t, got.Configuration.Spot.IsNull())
}

func TestMapEnvironmentClassToModel_DisabledClass(t *testing.T) {
	environmentClass := shared.EnvironmentClass{
		ID:          "env-class-disabled",
		RunnerID:    "runner-xyz",
		DisplayName: "Disabled Class",
		Description: "This class is disabled",
		Configuration: []shared.FieldValue{
			{Key: "instanceType", Value: "t3.micro"},
		},
		Enabled: false,
	}

	got := mapEnvironmentClassToModel(environmentClass)

	assert.Equal(t, "env-class-disabled", got.ID.ValueString())
	assert.False(t, got.Enabled.ValueBool())
	require.NotNil(t, got.Configuration)
	assert.Equal(t, "t3.micro", got.Configuration.InstanceType.ValueString())
}

func TestMapEnvironmentClassToModel_AWSEC2SpotInstance(t *testing.T) {
	environmentClass := shared.EnvironmentClass{
		ID:          "env-class-aws-spot",
		RunnerID:    "runner-aws-ec2",
		DisplayName: "Large Spot",
		Description: "8 vCPU / 32 GiB / 200 GiB disk (Spot)",
		Configuration: []shared.FieldValue{
			{Key: "instanceType", Value: "m7i.8xlarge"},
			{Key: "diskSizeGB", Value: "200"},
			{Key: "spot", Value: "true"},
		},
		Enabled: true,
	}

	got := mapEnvironmentClassToModel(environmentClass)

	assert.Equal(t, "env-class-aws-spot", got.ID.ValueString())
	assert.Equal(t, "runner-aws-ec2", got.RunnerID.ValueString())
	assert.Equal(t, "Large Spot", got.DisplayName.ValueString())
	assert.True(t, got.Enabled.ValueBool())

	require.NotNil(t, got.Configuration)
	assert.Equal(t, "m7i.8xlarge", got.Configuration.InstanceType.ValueString())
	assert.Equal(t, int64(200), got.Configuration.DiskSizeGB.ValueInt64())
	assert.True(t, got.Configuration.Spot.ValueBool())
}

func TestMapEnvironmentClassToModel_EmptyConfiguration(t *testing.T) {
	environmentClass := shared.EnvironmentClass{
		ID:            "env-class-no-config",
		RunnerID:      "runner-no-config",
		DisplayName:   "No Config",
		Configuration: []shared.FieldValue{},
		Enabled:       true,
	}

	got := mapEnvironmentClassToModel(environmentClass)

	assert.Nil(t, got.Configuration)
}

func TestMapEnvironmentClassToModel_NullDescription(t *testing.T) {
	environmentClass := shared.EnvironmentClass{
		ID:          "env-class-null-desc",
		RunnerID:    "runner-null-desc",
		DisplayName: "Has Name",
		Description: "",
		Configuration: []shared.FieldValue{
			{Key: "instanceType", Value: "t3.medium"},
		},
		Enabled: true,
	}

	got := mapEnvironmentClassToModel(environmentClass)

	assert.Equal(t, "Has Name", got.DisplayName.ValueString())
	assert.True(t, got.Description.IsNull())
	require.NotNil(t, got.Configuration)
	assert.Equal(t, "t3.medium", got.Configuration.InstanceType.ValueString())
}

func TestMapEnvironmentClassToModel_ManagedRunner(t *testing.T) {
	environmentClass := shared.EnvironmentClass{
		ID:          "env-class-managed",
		RunnerID:    "runner-managed",
		DisplayName: "Regular",
		Description: "4 vCPU / 16 GiB / 80 GiB disk",
		Configuration: []shared.FieldValue{
			{Key: "instanceType", Value: "m6i.xlarge"},
			{Key: "diskSizeGB", Value: "80"},
			{Key: "spot", Value: "false"},
		},
		Enabled: true,
	}

	got := mapEnvironmentClassToModel(environmentClass)

	assert.Equal(t, "env-class-managed", got.ID.ValueString())
	assert.Equal(t, "runner-managed", got.RunnerID.ValueString())
	assert.Equal(t, "Regular", got.DisplayName.ValueString())
	assert.Equal(t, "4 vCPU / 16 GiB / 80 GiB disk", got.Description.ValueString())

	require.NotNil(t, got.Configuration)
	assert.Equal(t, "m6i.xlarge", got.Configuration.InstanceType.ValueString())
	assert.Equal(t, int64(80), got.Configuration.DiskSizeGB.ValueInt64())
	assert.False(t, got.Configuration.Spot.ValueBool())
}

func TestMapEnvironmentClassToModel_ArbitraryKeysAccepted(t *testing.T) {
	// The API accepts arbitrary configuration keys without validation.
	// Invalid keys are silently ignored by the runner but stored in the API.
	environmentClass := shared.EnvironmentClass{
		ID:       "env-class-arbitrary",
		RunnerID: "runner-arbitrary",
		Configuration: []shared.FieldValue{
			{Key: "instanceType", Value: "t3.medium"},
			{Key: "customKey", Value: "customValue"},
			{Key: "anotherKey", Value: "anotherValue"},
		},
		Enabled: true,
	}

	got := mapEnvironmentClassToModel(environmentClass)

	require.NotNil(t, got.Configuration)

	// Valid key is mapped
	assert.Equal(t, "t3.medium", got.Configuration.InstanceType.ValueString())

	// Arbitrary keys are silently ignored by the mapping function
	// They are not part of the typed configuration model
	assert.True(t, got.Configuration.DiskSizeGB.IsNull())
	assert.True(t, got.Configuration.Spot.IsNull())
}
