package provider

import (
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvClassPlanWithID(t *testing.T) {
	t.Run("sets id and resolves unknown computed values to null", func(t *testing.T) {
		plan := runnerEnvironmentClassModel{
			RunnerID:    types.StringValue("runner-1"),
			DisplayName: types.StringValue("Small"),
			Description: types.StringUnknown(),
			Enabled:     types.BoolUnknown(),
			Configuration: &runnerEnvironmentClassConfigurationModel{
				InstanceType: types.StringValue("m6i.large"),
				DiskSizeGB:   types.Int64Unknown(),
				Spot:         types.BoolUnknown(),
			},
		}

		got := envClassPlanWithID(plan, "env-class-123")

		assert.Equal(t, "env-class-123", got.ID.ValueString())
		assert.Equal(t, "runner-1", got.RunnerID.ValueString())
		assert.True(t, got.Description.IsNull())
		assert.True(t, got.Enabled.IsNull())
		require.NotNil(t, got.Configuration)
		assert.Equal(t, "m6i.large", got.Configuration.InstanceType.ValueString())
		assert.True(t, got.Configuration.DiskSizeGB.IsNull())
		assert.True(t, got.Configuration.Spot.IsNull())
	})

	t.Run("preserves known values without mutating the plan's configuration", func(t *testing.T) {
		cfg := &runnerEnvironmentClassConfigurationModel{
			InstanceType: types.StringValue("t3.medium"),
			DiskSizeGB:   types.Int64Value(50),
			Spot:         types.BoolValue(true),
		}
		plan := runnerEnvironmentClassModel{
			DisplayName:   types.StringValue("Keep"),
			Description:   types.StringValue("desc"),
			Enabled:       types.BoolValue(false),
			Configuration: cfg,
		}

		got := envClassPlanWithID(plan, "env-class-x")

		assert.Equal(t, "desc", got.Description.ValueString())
		assert.False(t, got.Enabled.ValueBool())
		assert.Equal(t, int64(50), got.Configuration.DiskSizeGB.ValueInt64())
		assert.True(t, got.Configuration.Spot.ValueBool())
		assert.NotSame(t, cfg, got.Configuration)
	})
}

func TestMapEnvironmentClassToModel_AWSEC2Configuration(t *testing.T) {
	environmentClass := &v1.EnvironmentClass{
		Id:          "env-class-aws-ec2",
		RunnerId:    "runner-aws-ec2",
		DisplayName: "Large",
		Description: "8 vCPU / 32 GiB / 200 GiB disk",
		Configuration: []*v1.FieldValue{
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
	environmentClass := &v1.EnvironmentClass{
		Id:            "env-class-789",
		RunnerId:      "runner-abc",
		DisplayName:   "",
		Description:   "",
		Configuration: []*v1.FieldValue{},
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
	environmentClass := &v1.EnvironmentClass{
		Id:       "env-class-minimal",
		RunnerId: "runner-aws-ec2",
		Configuration: []*v1.FieldValue{
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
	environmentClass := &v1.EnvironmentClass{
		Id:          "env-class-disabled",
		RunnerId:    "runner-xyz",
		DisplayName: "Disabled Class",
		Description: "This class is disabled",
		Configuration: []*v1.FieldValue{
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
	environmentClass := &v1.EnvironmentClass{
		Id:          "env-class-aws-spot",
		RunnerId:    "runner-aws-ec2",
		DisplayName: "Large Spot",
		Description: "8 vCPU / 32 GiB / 200 GiB disk (Spot)",
		Configuration: []*v1.FieldValue{
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
	environmentClass := &v1.EnvironmentClass{
		Id:            "env-class-no-config",
		RunnerId:      "runner-no-config",
		DisplayName:   "No Config",
		Configuration: []*v1.FieldValue{},
		Enabled:       true,
	}

	got := mapEnvironmentClassToModel(environmentClass)

	assert.Nil(t, got.Configuration)
}

func TestMapEnvironmentClassToModel_NullDescription(t *testing.T) {
	environmentClass := &v1.EnvironmentClass{
		Id:          "env-class-null-desc",
		RunnerId:    "runner-null-desc",
		DisplayName: "Has Name",
		Description: "",
		Configuration: []*v1.FieldValue{
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
	environmentClass := &v1.EnvironmentClass{
		Id:          "env-class-managed",
		RunnerId:    "runner-managed",
		DisplayName: "Regular",
		Description: "4 vCPU / 16 GiB / 80 GiB disk",
		Configuration: []*v1.FieldValue{
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
	environmentClass := &v1.EnvironmentClass{
		Id:       "env-class-arbitrary",
		RunnerId: "runner-arbitrary",
		Configuration: []*v1.FieldValue{
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
