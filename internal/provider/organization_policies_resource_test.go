package provider

import (
	"context"
	"sort"
	"testing"
	"time"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	frameworkdiag "github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestOrganizationPoliciesResourceSchema(t *testing.T) {
	r := &organizationPoliciesResource{}

	var metadata resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "ona"}, &metadata)
	assert.Equal(t, "ona_organization_policies", metadata.TypeName)

	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)

	require.False(t, resp.Diagnostics.HasError())
	attributeNames := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		attributeNames = append(attributeNames, name)
	}
	sort.Strings(attributeNames)
	assert.Equal(t, []string{
		"allow_local_runners",
		"id",
		"maximum_environment_timeout",
		"maximum_environments_per_user",
		"maximum_running_environments_per_user",
		"members_create_projects",
		"members_require_projects",
		"port_sharing_disabled",
	}, attributeNames)

	for _, name := range []string{
		"allow_local_runners",
		"maximum_environment_timeout",
		"maximum_environments_per_user",
		"maximum_running_environments_per_user",
		"members_create_projects",
		"members_require_projects",
		"port_sharing_disabled",
	} {
		attribute := resp.Schema.Attributes[name]
		assert.Truef(t, attribute.IsOptional(), "%s should be optional", name)
		assert.Truef(t, attribute.IsComputed(), "%s should be computed", name)
	}
}

func TestBuildOrganizationPoliciesUpdateRequest_OmitsUnconfiguredFields(t *testing.T) {
	config := organizationPoliciesModel{
		ID:                                types.StringNull(),
		MaximumEnvironmentTimeout:         types.StringValue("45m"),
		MembersRequireProjects:            types.BoolNull(),
		MembersCreateProjects:             types.BoolNull(),
		AllowLocalRunners:                 types.BoolValue(false),
		MaximumRunningEnvironmentsPerUser: types.Int64Null(),
		MaximumEnvironmentsPerUser:        types.Int64Value(12),
		PortSharingDisabled:               types.BoolNull(),
	}

	got, diags := buildOrganizationPoliciesUpdateRequest("org-1", config)

	require.False(t, diags.HasError(), diags.Errors())
	assert.Equal(t, "org-1", got.GetOrganizationId())
	assert.Equal(t, 45*time.Minute, got.GetMaximumEnvironmentTimeout().AsDuration())
	require.NotNil(t, got.AllowLocalRunners)
	assert.False(t, got.GetAllowLocalRunners())
	require.NotNil(t, got.MaximumEnvironmentsPerUser)
	assert.Equal(t, int64(12), got.GetMaximumEnvironmentsPerUser())
	assert.Nil(t, got.MembersRequireProjects)
	assert.Nil(t, got.MembersCreateProjects)
	assert.Nil(t, got.MaximumRunningEnvironmentsPerUser)
	assert.Nil(t, got.PortSharingDisabled)
}

func TestValidateOrganizationPoliciesModel(t *testing.T) {
	tests := []struct {
		name        string
		config      organizationPoliciesModel
		wantSummary string
	}{
		{
			name: "valid",
			config: organizationPoliciesModel{
				MaximumEnvironmentTimeout: types.StringValue("30m"),
				MembersRequireProjects:    types.BoolValue(true),
				MembersCreateProjects:     types.BoolValue(false),
			},
		},
		{
			name: "duration below minimum",
			config: organizationPoliciesModel{
				MaximumEnvironmentTimeout: types.StringValue("29m"),
				MembersRequireProjects:    types.BoolNull(),
				MembersCreateProjects:     types.BoolNull(),
			},
			wantSummary: "Invalid Duration",
		},
		{
			name: "project fields must be paired",
			config: organizationPoliciesModel{
				MaximumEnvironmentTimeout: types.StringNull(),
				MembersRequireProjects:    types.BoolValue(true),
				MembersCreateProjects:     types.BoolNull(),
			},
			wantSummary: "Invalid Project Member Policy",
		},
		{
			name: "project fields must be opposite",
			config: organizationPoliciesModel{
				MaximumEnvironmentTimeout: types.StringNull(),
				MembersRequireProjects:    types.BoolValue(true),
				MembersCreateProjects:     types.BoolValue(true),
			},
			wantSummary: "Invalid Project Member Policy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := validateOrganizationPoliciesModel(tt.config)
			if tt.wantSummary == "" {
				assert.False(t, diags.HasError(), diags.Errors())
				return
			}

			require.True(t, diags.HasError())
			assert.Equal(t, tt.wantSummary, diags.Errors()[0].Summary())
		})
	}
}

func TestMapOrganizationPoliciesToModel(t *testing.T) {
	prior := organizationPoliciesModel{MaximumEnvironmentTimeout: types.StringValue("1h0m0s")}
	policies := &v1.OrganizationPolicies{
		OrganizationId:                    "org-1",
		MaximumEnvironmentTimeout:         durationpb.New(time.Hour),
		MembersRequireProjects:            true,
		MembersCreateProjects:             false,
		AllowLocalRunners:                 false,
		MaximumRunningEnvironmentsPerUser: 4,
		MaximumEnvironmentsPerUser:        10,
		PortSharingDisabled:               true,
	}

	got := mapOrganizationPoliciesToModel(policies, prior)

	assert.Equal(t, "org-1", got.ID.ValueString())
	assert.Equal(t, "1h0m0s", got.MaximumEnvironmentTimeout.ValueString())
	assert.True(t, got.MembersRequireProjects.ValueBool())
	assert.False(t, got.MembersCreateProjects.ValueBool())
	assert.False(t, got.AllowLocalRunners.ValueBool())
	assert.Equal(t, int64(4), got.MaximumRunningEnvironmentsPerUser.ValueInt64())
	assert.Equal(t, int64(10), got.MaximumEnvironmentsPerUser.ValueInt64())
	assert.True(t, got.PortSharingDisabled.ValueBool())
}

func TestOrganizationPoliciesBaselineRoundTripAndRestore(t *testing.T) {
	state := &fakeOrganizationPoliciesPrivateState{data: make(map[string][]byte)}
	config := organizationPoliciesModel{
		MaximumEnvironmentTimeout:  types.StringValue("2h"),
		AllowLocalRunners:          types.BoolValue(false),
		MaximumEnvironmentsPerUser: types.Int64Value(20),
		PortSharingDisabled:        types.BoolValue(true),
	}
	baseline := &v1.OrganizationPolicies{
		OrganizationId:                    "org-1",
		MaximumEnvironmentTimeout:         durationpb.New(2 * time.Hour),
		MembersRequireProjects:            true,
		MembersCreateProjects:             false,
		AllowLocalRunners:                 false,
		MaximumRunningEnvironmentsPerUser: 5,
		MaximumEnvironmentsPerUser:        20,
		PortSharingDisabled:               true,
		DefaultEditorId:                   "not-managed-by-this-resource",
	}

	diags := setOrganizationPoliciesBaseline(context.Background(), state, baseline, config)
	require.False(t, diags.HasError(), diags.Errors())

	stored, diags := getOrganizationPoliciesBaseline(context.Background(), state)
	require.False(t, diags.HasError(), diags.Errors())

	restore := restoreOrganizationPoliciesRequest("org-1", stored)
	assert.Equal(t, 2*time.Hour, restore.GetMaximumEnvironmentTimeout().AsDuration())
	assert.False(t, restore.GetAllowLocalRunners())
	assert.Equal(t, int64(20), restore.GetMaximumEnvironmentsPerUser())
	assert.True(t, restore.GetPortSharingDisabled())
	assert.Nil(t, restore.MembersRequireProjects)
	assert.Nil(t, restore.MembersCreateProjects)
	assert.Nil(t, restore.MaximumRunningEnvironmentsPerUser)
	assert.Nil(t, restore.DefaultEditorId)
}

func TestMergeOrganizationPoliciesBaseline_PreservesOriginalValues(t *testing.T) {
	state := &fakeOrganizationPoliciesPrivateState{data: make(map[string][]byte)}
	initial := &v1.OrganizationPolicies{
		MaximumEnvironmentsPerUser: 10,
		PortSharingDisabled:        false,
	}
	diags := setOrganizationPoliciesBaseline(context.Background(), state, initial, organizationPoliciesModel{
		MaximumEnvironmentsPerUser: types.Int64Value(20),
	})
	require.False(t, diags.HasError(), diags.Errors())

	beforeSecondUpdate := &v1.OrganizationPolicies{
		MaximumEnvironmentsPerUser: 20,
		PortSharingDisabled:        true,
	}
	diags = mergeOrganizationPoliciesBaseline(context.Background(), state, state, beforeSecondUpdate, organizationPoliciesModel{
		MaximumEnvironmentsPerUser: types.Int64Value(30),
		PortSharingDisabled:        types.BoolValue(false),
	})
	require.False(t, diags.HasError(), diags.Errors())

	baseline, diags := getOrganizationPoliciesBaseline(context.Background(), state)
	require.False(t, diags.HasError(), diags.Errors())
	restore := restoreOrganizationPoliciesRequest("org-1", baseline)
	require.NotNil(t, restore.MaximumEnvironmentsPerUser)
	assert.Equal(t, int64(10), restore.GetMaximumEnvironmentsPerUser())
	require.NotNil(t, restore.PortSharingDisabled)
	assert.True(t, restore.GetPortSharingDisabled())
}

type fakeOrganizationPoliciesPrivateState struct {
	data map[string][]byte
}

func (s *fakeOrganizationPoliciesPrivateState) GetKey(_ context.Context, key string) ([]byte, frameworkdiag.Diagnostics) {
	return s.data[key], nil
}

func (s *fakeOrganizationPoliciesPrivateState) SetKey(_ context.Context, key string, value []byte) frameworkdiag.Diagnostics {
	s.data[key] = value
	return nil
}
