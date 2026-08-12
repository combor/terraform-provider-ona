package provider

import (
	"context"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &securityPolicyResource{}
	_ resource.ResourceWithImportState = &securityPolicyResource{}
)

type securityPolicyResource struct {
	client *sdk.Client
}

func NewSecurityPolicyResource() resource.Resource {
	return &securityPolicyResource{}
}

type securityPolicyModel struct {
	ID             types.String                    `tfsdk:"id"`
	Name           types.String                    `tfsdk:"name"`
	OrganizationID types.String                    `tfsdk:"organization_id"`
	Ports          *securityPolicyPortsModel       `tfsdk:"ports"`
	Executables    *securityPolicyExecutablesModel `tfsdk:"executables"`
	CreatedAt      types.String                    `tfsdk:"created_at"`
	UpdatedAt      types.String                    `tfsdk:"updated_at"`
}

type securityPolicyPortsModel struct {
	MaxAdmissionLevel types.String `tfsdk:"max_admission_level"`
}

type securityPolicyExecutablesModel struct {
	DefaultEffect types.String              `tfsdk:"default_effect"`
	Rules         []securityPolicyRuleModel `tfsdk:"rules"`
}

type securityPolicyRuleModel struct {
	Path   types.String `tfsdk:"path"`
	Effect types.String `tfsdk:"effect"`
}

func (r *securityPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_policy"
}

func (r *securityPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a security policy: the port and executable guardrails applied to environments the policy is assigned to.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Security policy ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Security policy name.",
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization ID the policy belongs to. Resolved from the authenticated identity.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			// Optional without Computed: Terraform proposes the prior state for a
			// removed optional-and-computed attribute, which would make a
			// guardrail impossible to clear once it had been set.
			"ports": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Port guardrails.",
				Attributes: map[string]schema.Attribute{
					"max_admission_level": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Caps how widely a user may share a port: `ADMISSION_LEVEL_OWNER_ONLY`, `ADMISSION_LEVEL_CREATOR_ONLY`, `ADMISSION_LEVEL_ORGANIZATION` or `ADMISSION_LEVEL_EVERYONE`.",
						Validators:          enumValidators(v1.AdmissionLevel_value),
					},
				},
			},
			"executables": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Executable guardrails.",
				Attributes: map[string]schema.Attribute{
					// Read-only: the API accepts no effect other than the
					// EFFECT_ALLOW it normalises an omitted value to.
					"default_effect": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Effect applied to executables that match no rule. Always `EFFECT_ALLOW`.",
					},
					// A set, not a list: the API attaches no meaning to rule order
					// (conflicting decisions resolve to block, not to the first
					// match), and a response that reordered the rules would not
					// match a planned list.
					"rules": schema.SetNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Per-executable decisions.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"path": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "Absolute executable path such as `/usr/bin/curl`, or a bare executable name such as `npx`.",
								},
								"effect": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "Decision for this executable: `EFFECT_AUDIT` or `EFFECT_BLOCK`. `EFFECT_ALLOW` is not accepted on a rule.",
									Validators:          enumValidators(ruleEffectValues),
								},
							},
						},
					},
				},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the policy was created (RFC3339).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the policy was last updated (RFC3339).",
			},
		},
	}
}

func (r *securityPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.client = client
}

func (r *securityPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan securityPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationID, err := authenticatedOrganizationID(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to resolve organization", err.Error())
		return
	}

	spec := buildSecurityPolicySpec(plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := r.client.Services.Security.CreateSecurityPolicy(ctx, connect.NewRequest(&v1.CreateSecurityPolicyRequest{
		OrganizationId: organizationID,
		Metadata:       &v1.SecurityPolicy_Metadata{Name: plan.Name.ValueString()},
		Spec:           spec,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create security policy", err.Error())
		return
	}

	state := mapSecurityPolicyToModel(createResp.Msg.GetSecurityPolicy(), plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *securityPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state securityPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Services.Security.GetSecurityPolicy(ctx, connect.NewRequest(&v1.GetSecurityPolicyRequest{
		SecurityPolicyId: state.ID.ValueString(),
	}))
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read security policy", err.Error())
		return
	}

	newState := mapSecurityPolicyToModel(getResp.Msg.GetSecurityPolicy(), state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *securityPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan securityPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var prior securityPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	spec := buildSecurityPolicySpec(plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Metadata and spec are sent in full: the API replaces what it is given, so
	// a rule removed from the configuration has to be absent from the request.
	updateResp, err := r.client.Services.Security.UpdateSecurityPolicy(ctx, connect.NewRequest(&v1.UpdateSecurityPolicyRequest{
		SecurityPolicyId: prior.ID.ValueString(),
		Metadata:         &v1.SecurityPolicy_Metadata{Name: plan.Name.ValueString()},
		Spec:             spec,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update security policy", err.Error())
		return
	}

	state := mapSecurityPolicyToModel(updateResp.Msg.GetSecurityPolicy(), plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *securityPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state securityPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Services.Security.DeleteSecurityPolicy(ctx, connect.NewRequest(&v1.DeleteSecurityPolicyRequest{
		SecurityPolicyId: state.ID.ValueString(),
	}))
	if err != nil {
		if isAPINotFound(err) {
			return
		}

		resp.Diagnostics.AddError("Failed to delete security policy", err.Error())
	}
}

func (r *securityPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildSecurityPolicySpec converts the configured guardrails into the spec the
// API expects. The spec itself is always sent — it is a required field — while
// its two sub-policies are omitted when the configuration leaves them out.
func buildSecurityPolicySpec(plan securityPolicyModel, diagnostics *diag.Diagnostics) *v1.SecurityPolicy_Spec {
	spec := &v1.SecurityPolicy_Spec{}

	if ports := plan.Ports; ports != nil {
		spec.Ports = &v1.SecurityPolicy_Spec_PortPolicy{}
		if isKnownString(ports.MaxAdmissionLevel) {
			spec.Ports.MaxAdmissionLevel = enumValue[v1.AdmissionLevel]("max_admission_level",
				ports.MaxAdmissionLevel.ValueString(), v1.AdmissionLevel_value, diagnostics)
		}
	}

	if executables := plan.Executables; executables != nil {
		// default_effect is read-only, so the API decides it; only the rules are
		// sent back.
		spec.Executables = &v1.SecurityPolicy_Spec_ExecutablePolicy{}
		for _, rule := range executables.Rules {
			spec.Executables.Rules = append(spec.Executables.Rules, &v1.SecurityPolicy_Spec_ExecutablePolicy_Rule{
				Path:   rule.Path.ValueString(),
				Effect: securityPolicyRuleEffect(rule.Effect, diagnostics),
			})
		}
	}

	return spec
}

// ruleEffectValues are the decisions a rule may carry. EFFECT_ALLOW is a valid
// enum value elsewhere but not on a rule, so it is left out here and reported
// like any other unrecognised effect.
var ruleEffectValues = map[string]int32{
	v1.SecurityPolicy_EFFECT_AUDIT.String(): int32(v1.SecurityPolicy_EFFECT_AUDIT),
	v1.SecurityPolicy_EFFECT_BLOCK.String(): int32(v1.SecurityPolicy_EFFECT_BLOCK),
}

func securityPolicyRuleEffect(effect types.String, diagnostics *diag.Diagnostics) v1.SecurityPolicy_Effect {
	return enumValue[v1.SecurityPolicy_Effect]("effect", effect.ValueString(), ruleEffectValues, diagnostics)
}

func isKnownString(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown()
}

// mapSecurityPolicyToModel converts an API policy into resource state. prior is
// the plan on create and update, and the previous state on refresh; it is only
// consulted to tell a configured-but-empty rule list from an absent one, which
// the API reports identically.
func mapSecurityPolicyToModel(policy *v1.SecurityPolicy, prior securityPolicyModel) securityPolicyModel {
	model := securityPolicyModel{
		ID:             types.StringValue(policy.GetId()),
		Name:           stringValueOrNull(policy.GetMetadata().GetName()),
		OrganizationID: stringValueOrNull(policy.GetOrganizationId()),
		CreatedAt:      timeValueOrNull(policy.GetCreatedAt()),
		UpdatedAt:      timeValueOrNull(policy.GetUpdatedAt()),
	}

	if ports := policy.GetSpec().GetPorts(); ports != nil {
		model.Ports = &securityPolicyPortsModel{
			MaxAdmissionLevel: stringValueOrNull(enumString(ports.GetMaxAdmissionLevel())),
		}
	}

	if executables := policy.GetSpec().GetExecutables(); executables != nil {
		rules := priorRuleList(prior, len(executables.GetRules()))
		for _, rule := range executables.GetRules() {
			rules = append(rules, securityPolicyRuleModel{
				Path:   stringValueOrNull(rule.GetPath()),
				Effect: stringValueOrNull(enumString(rule.GetEffect())),
			})
		}
		model.Executables = &securityPolicyExecutablesModel{
			DefaultEffect: stringValueOrNull(enumString(executables.GetDefaultEffect())),
			Rules:         rules,
		}
	}

	return model
}

// priorRuleList seeds the rule slice so that an API response with no rules maps
// back to whatever the configuration said: an empty list stays empty, an
// omitted list stays null.
func priorRuleList(prior securityPolicyModel, count int) []securityPolicyRuleModel {
	if count == 0 && (prior.Executables == nil || prior.Executables.Rules == nil) {
		return nil
	}

	return make([]securityPolicyRuleModel, 0, count)
}
