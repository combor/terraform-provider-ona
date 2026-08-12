package provider

import (
	"testing"
	"time"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBuildSecurityPolicySpec(t *testing.T) {
	t.Run("omits sub-policies the configuration leaves out", func(t *testing.T) {
		var diags diag.Diagnostics
		got := buildSecurityPolicySpec(securityPolicyModel{}, &diags)
		require.False(t, diags.HasError())

		require.NotNil(t, got)
		assert.Nil(t, got.GetPorts())
		assert.Nil(t, got.GetExecutables())
	})

	t.Run("converts ports and executable rules", func(t *testing.T) {
		var diags diag.Diagnostics
		got := buildSecurityPolicySpec(securityPolicyModel{
			Ports: &securityPolicyPortsModel{
				MaxAdmissionLevel: types.StringValue("ADMISSION_LEVEL_ORGANIZATION"),
			},
			Executables: &securityPolicyExecutablesModel{
				Rules: []securityPolicyRuleModel{
					{Path: types.StringValue("/usr/bin/curl"), Effect: types.StringValue("EFFECT_BLOCK")},
					{Path: types.StringValue("npx"), Effect: types.StringValue("EFFECT_AUDIT")},
				},
			},
		}, &diags)
		require.False(t, diags.HasError())

		assert.Equal(t, v1.AdmissionLevel_ADMISSION_LEVEL_ORGANIZATION, got.GetPorts().GetMaxAdmissionLevel())
		require.Len(t, got.GetExecutables().GetRules(), 2)
		assert.Equal(t, "/usr/bin/curl", got.GetExecutables().GetRules()[0].GetPath())
		assert.Equal(t, v1.SecurityPolicy_EFFECT_BLOCK, got.GetExecutables().GetRules()[0].GetEffect())
		assert.Equal(t, "npx", got.GetExecutables().GetRules()[1].GetPath())
		assert.Equal(t, v1.SecurityPolicy_EFFECT_AUDIT, got.GetExecutables().GetRules()[1].GetEffect())
	})

	t.Run("keeps an empty executables policy so removing every rule clears them", func(t *testing.T) {
		var diags diag.Diagnostics
		got := buildSecurityPolicySpec(securityPolicyModel{
			Executables: &securityPolicyExecutablesModel{},
		}, &diags)
		require.False(t, diags.HasError())

		require.NotNil(t, got.GetExecutables())
		assert.Empty(t, got.GetExecutables().GetRules())
	})

	t.Run("never sends the read-only default effect", func(t *testing.T) {
		var diags diag.Diagnostics
		got := buildSecurityPolicySpec(securityPolicyModel{
			Executables: &securityPolicyExecutablesModel{
				DefaultEffect: types.StringValue("EFFECT_ALLOW"),
			},
		}, &diags)
		require.False(t, diags.HasError())

		assert.Equal(t, v1.SecurityPolicy_EFFECT_UNSPECIFIED, got.GetExecutables().GetDefaultEffect())
	})

	t.Run("rejects EFFECT_ALLOW on a rule", func(t *testing.T) {
		var diags diag.Diagnostics
		buildSecurityPolicySpec(securityPolicyModel{
			Executables: &securityPolicyExecutablesModel{
				Rules: []securityPolicyRuleModel{
					{Path: types.StringValue("npx"), Effect: types.StringValue("EFFECT_ALLOW")},
				},
			},
		}, &diags)

		require.True(t, diags.HasError())
		assert.Contains(t, diags.Errors()[0].Detail(), "EFFECT_AUDIT")
		assert.Contains(t, diags.Errors()[0].Detail(), "EFFECT_BLOCK")
	})

	t.Run("reports an unknown effect instead of sending unspecified", func(t *testing.T) {
		var diags diag.Diagnostics
		buildSecurityPolicySpec(securityPolicyModel{
			Executables: &securityPolicyExecutablesModel{
				Rules: []securityPolicyRuleModel{
					{Path: types.StringValue("npx"), Effect: types.StringValue("block")},
				},
			},
		}, &diags)

		require.True(t, diags.HasError())
		assert.Contains(t, diags.Errors()[0].Detail(), "EFFECT_BLOCK")
	})
}

func TestMapSecurityPolicyToModel(t *testing.T) {
	policy := &v1.SecurityPolicy{
		Id:             "policy-1",
		OrganizationId: "org-1",
		Metadata:       &v1.SecurityPolicy_Metadata{Name: "restricted"},
		Spec: &v1.SecurityPolicy_Spec{
			Ports: &v1.SecurityPolicy_Spec_PortPolicy{
				MaxAdmissionLevel: v1.AdmissionLevel_ADMISSION_LEVEL_ORGANIZATION,
			},
			Executables: &v1.SecurityPolicy_Spec_ExecutablePolicy{
				DefaultEffect: v1.SecurityPolicy_EFFECT_ALLOW,
				Rules: []*v1.SecurityPolicy_Spec_ExecutablePolicy_Rule{
					{Path: "/usr/bin/curl", Effect: v1.SecurityPolicy_EFFECT_BLOCK},
				},
			},
		},
		CreatedAt: timestamppb.New(time.Date(2026, time.March, 2, 15, 4, 5, 0, time.UTC)),
		UpdatedAt: timestamppb.New(time.Date(2026, time.March, 3, 15, 4, 5, 0, time.UTC)),
	}

	got := mapSecurityPolicyToModel(policy, securityPolicyModel{})

	assert.Equal(t, "policy-1", got.ID.ValueString())
	assert.Equal(t, "restricted", got.Name.ValueString())
	assert.Equal(t, "org-1", got.OrganizationID.ValueString())
	require.NotNil(t, got.Ports)
	assert.Equal(t, "ADMISSION_LEVEL_ORGANIZATION", got.Ports.MaxAdmissionLevel.ValueString())
	require.NotNil(t, got.Executables)
	assert.Equal(t, "EFFECT_ALLOW", got.Executables.DefaultEffect.ValueString())
	require.Len(t, got.Executables.Rules, 1)
	assert.Equal(t, "/usr/bin/curl", got.Executables.Rules[0].Path.ValueString())
	assert.Equal(t, "EFFECT_BLOCK", got.Executables.Rules[0].Effect.ValueString())
	assert.Equal(t, "2026-03-02T15:04:05Z", got.CreatedAt.ValueString())
	assert.Equal(t, "2026-03-03T15:04:05Z", got.UpdatedAt.ValueString())
}

func TestMapSecurityPolicyToModel_AbsentSubPoliciesStayNull(t *testing.T) {
	got := mapSecurityPolicyToModel(&v1.SecurityPolicy{
		Id:       "policy-1",
		Metadata: &v1.SecurityPolicy_Metadata{Name: "empty"},
		Spec:     &v1.SecurityPolicy_Spec{},
	}, securityPolicyModel{})

	assert.Nil(t, got.Ports)
	assert.Nil(t, got.Executables)
}

func TestMapSecurityPolicyToModel_RuleListFollowsThePlan(t *testing.T) {
	policy := &v1.SecurityPolicy{
		Id:       "policy-1",
		Metadata: &v1.SecurityPolicy_Metadata{Name: "empty"},
		Spec: &v1.SecurityPolicy_Spec{
			Executables: &v1.SecurityPolicy_Spec_ExecutablePolicy{
				DefaultEffect: v1.SecurityPolicy_EFFECT_ALLOW,
			},
		},
	}

	t.Run("an omitted rule list stays null", func(t *testing.T) {
		got := mapSecurityPolicyToModel(policy, securityPolicyModel{
			Executables: &securityPolicyExecutablesModel{},
		})
		require.NotNil(t, got.Executables)
		assert.Nil(t, got.Executables.Rules)
	})

	t.Run("an explicitly empty rule list stays empty", func(t *testing.T) {
		got := mapSecurityPolicyToModel(policy, securityPolicyModel{
			Executables: &securityPolicyExecutablesModel{Rules: []securityPolicyRuleModel{}},
		})
		require.NotNil(t, got.Executables)
		require.NotNil(t, got.Executables.Rules)
		assert.Empty(t, got.Executables.Rules)
	})
}
