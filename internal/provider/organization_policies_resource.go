package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	"google.golang.org/protobuf/types/known/durationpb"
)

var (
	_ resource.Resource                   = &organizationPoliciesResource{}
	_ resource.ResourceWithConfigure      = &organizationPoliciesResource{}
	_ resource.ResourceWithImportState    = &organizationPoliciesResource{}
	_ resource.ResourceWithValidateConfig = &organizationPoliciesResource{}
)

const organizationPoliciesBaselinePrivateKey = "organization_policies_baseline"

type organizationPoliciesResource struct {
	client *sdk.Client
}

type organizationPoliciesModel struct {
	ID                                types.String `tfsdk:"id"`
	MaximumEnvironmentTimeout         types.String `tfsdk:"maximum_environment_timeout"`
	MembersRequireProjects            types.Bool   `tfsdk:"members_require_projects"`
	MembersCreateProjects             types.Bool   `tfsdk:"members_create_projects"`
	AllowLocalRunners                 types.Bool   `tfsdk:"allow_local_runners"`
	MaximumRunningEnvironmentsPerUser types.Int64  `tfsdk:"maximum_running_environments_per_user"`
	MaximumEnvironmentsPerUser        types.Int64  `tfsdk:"maximum_environments_per_user"`
	PortSharingDisabled               types.Bool   `tfsdk:"port_sharing_disabled"`
}

type organizationPoliciesPrivateState interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

type organizationPoliciesBaseline struct {
	MaximumEnvironmentTimeout         *durationpb.Duration `json:"maximum_environment_timeout,omitempty"`
	MembersRequireProjects            *bool                `json:"members_require_projects,omitempty"`
	MembersCreateProjects             *bool                `json:"members_create_projects,omitempty"`
	AllowLocalRunners                 *bool                `json:"allow_local_runners,omitempty"`
	MaximumRunningEnvironmentsPerUser *int64               `json:"maximum_running_environments_per_user,omitempty"`
	MaximumEnvironmentsPerUser        *int64               `json:"maximum_environments_per_user,omitempty"`
	PortSharingDisabled               *bool                `json:"port_sharing_disabled,omitempty"`
}

func NewOrganizationPoliciesResource() resource.Resource {
	return &organizationPoliciesResource{}
}

func (r *organizationPoliciesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_policies"
}

func (r *organizationPoliciesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages selected organization policies for the organization associated with the authenticated provider token. Omitted attributes remain unmanaged. Destroying the resource restores the original values of attributes changed through this resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Authenticated organization ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"maximum_environment_timeout": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Maximum timeout allowed for environments as a Go duration, such as `30m`, `2h`, or `0s`. Zero means no limit; non-zero values must be at least 30 minutes. Omit to leave unmanaged.",
			},
			"members_require_projects": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether non-admin users can only create environments from projects. Configure together with `members_create_projects`; their values must be opposite.",
			},
			"members_create_projects": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether organization members can create projects. Configure together with `members_require_projects`; their values must be opposite.",
			},
			"allow_local_runners": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether local runners are allowed. The Ona API currently rejects enabling local runners through organization policies. Omit to leave unmanaged.",
			},
			"maximum_running_environments_per_user": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Maximum simultaneously running environments per user. Omit to leave unmanaged.",
			},
			"maximum_environments_per_user": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Maximum total environments per user. Omit to leave unmanaged.",
			},
			"port_sharing_disabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether user-initiated port sharing is disabled. System ports remain exempt. Omit to leave unmanaged.",
			},
		},
	}
}

func (r *organizationPoliciesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.client = client
}

func (r *organizationPoliciesResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config organizationPoliciesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateOrganizationPoliciesModel(config)...)
}

func (r *organizationPoliciesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var config organizationPoliciesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan organizationPoliciesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to resolve authenticated organization", err.Error())
		return
	}

	current, err := r.getOrganizationPolicies(ctx, organizationID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read organization policies before update", err.Error())
		return
	}
	resp.Diagnostics.Append(setOrganizationPoliciesBaseline(ctx, resp.Private, current, config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateRequest, diags := buildOrganizationPoliciesUpdateRequest(organizationID, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if hasOrganizationPolicyUpdates(updateRequest) {
		if _, err := r.client.Services.Organization.UpdateOrganizationPolicies(ctx, connect.NewRequest(updateRequest)); err != nil {
			resp.Diagnostics.AddError("Failed to update organization policies", err.Error())
			return
		}
	}

	fallback := fallbackOrganizationPoliciesState(current, updateRequest, plan)
	fallback.ID = types.StringValue(organizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &fallback)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.getOrganizationPolicies(ctx, organizationID)
	if err != nil {
		resp.Diagnostics.AddWarning("Could not read organization policies after update", err.Error())
		return
	}
	state := mapOrganizationPoliciesToModel(updated, plan)
	state.ID = types.StringValue(organizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *organizationPoliciesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior organizationPoliciesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to resolve authenticated organization", err.Error())
		return
	}
	if !guardOrganizationPoliciesID(prior.ID, organizationID, &resp.Diagnostics) {
		return
	}

	policies, err := r.getOrganizationPolicies(ctx, organizationID)
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read organization policies", err.Error())
		return
	}
	resp.Diagnostics.Append(ensureOrganizationPoliciesBaseline(ctx, req.Private, resp.Private, policies)...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := mapOrganizationPoliciesToModel(policies, prior)
	state.ID = types.StringValue(organizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *organizationPoliciesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var config organizationPoliciesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan organizationPoliciesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var prior organizationPoliciesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to resolve authenticated organization", err.Error())
		return
	}
	if !guardOrganizationPoliciesID(prior.ID, organizationID, &resp.Diagnostics) {
		return
	}

	current, err := r.getOrganizationPolicies(ctx, organizationID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read organization policies before update", err.Error())
		return
	}
	resp.Diagnostics.Append(mergeOrganizationPoliciesBaseline(ctx, req.Private, resp.Private, current, config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateRequest, diags := buildOrganizationPoliciesUpdateRequest(organizationID, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if hasOrganizationPolicyUpdates(updateRequest) {
		if _, err := r.client.Services.Organization.UpdateOrganizationPolicies(ctx, connect.NewRequest(updateRequest)); err != nil {
			resp.Diagnostics.AddError("Failed to update organization policies", err.Error())
			return
		}
	}

	fallback := fallbackOrganizationPoliciesState(current, updateRequest, plan)
	fallback.ID = types.StringValue(organizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &fallback)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.getOrganizationPolicies(ctx, organizationID)
	if err != nil {
		resp.Diagnostics.AddWarning("Could not read organization policies after update", err.Error())
		return
	}
	state := mapOrganizationPoliciesToModel(updated, plan)
	state.ID = types.StringValue(organizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *organizationPoliciesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state organizationPoliciesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to resolve authenticated organization", err.Error())
		return
	}
	if !guardOrganizationPoliciesID(state.ID, organizationID, &resp.Diagnostics) {
		return
	}

	baseline, diags := getOrganizationPoliciesBaseline(ctx, req.Private)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	restoreRequest := restoreOrganizationPoliciesRequest(organizationID, baseline)
	if hasOrganizationPolicyUpdates(restoreRequest) {
		if _, err := r.client.Services.Organization.UpdateOrganizationPolicies(ctx, connect.NewRequest(restoreRequest)); err != nil {
			resp.Diagnostics.AddError("Failed to restore organization policies", err.Error())
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *organizationPoliciesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to resolve authenticated organization", err.Error())
		return
	}
	if req.ID != "current" && req.ID != organizationID {
		resp.Diagnostics.AddError(
			"Invalid organization policies import ID",
			fmt.Sprintf("Import ona_organization_policies with \"current\" or the authenticated organization ID %q.", organizationID),
		)
		return
	}

	policies, err := r.getOrganizationPolicies(ctx, organizationID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read organization policies during import", err.Error())
		return
	}
	resp.Diagnostics.Append(setOrganizationPoliciesBaseline(ctx, resp.Private, policies, organizationPoliciesModel{})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), organizationID)...)
}

func (r *organizationPoliciesResource) authenticatedOrganizationID(ctx context.Context) (string, error) {
	return authenticatedOrganizationID(ctx, r.client)
}

func (r *organizationPoliciesResource) getOrganizationPolicies(ctx context.Context, organizationID string) (*v1.OrganizationPolicies, error) {
	result, err := r.client.Services.Organization.GetOrganizationPolicies(ctx, connect.NewRequest(&v1.GetOrganizationPoliciesRequest{
		OrganizationId: organizationID,
	}))
	if err != nil {
		return nil, fmt.Errorf("get organization policies: %w", err)
	}
	policies := result.Msg.GetPolicies()
	if policies == nil {
		return nil, fmt.Errorf("get organization policies: API returned an empty policy object")
	}
	return policies, nil
}

func buildOrganizationPoliciesUpdateRequest(organizationID string, config organizationPoliciesModel) (*v1.UpdateOrganizationPoliciesRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	request := &v1.UpdateOrganizationPoliciesRequest{OrganizationId: organizationID}

	if !config.MaximumEnvironmentTimeout.IsNull() && !config.MaximumEnvironmentTimeout.IsUnknown() {
		duration, durationDiags := parseMaximumEnvironmentTimeout(config.MaximumEnvironmentTimeout)
		diags.Append(durationDiags...)
		request.MaximumEnvironmentTimeout = duration
	}
	if !config.MembersRequireProjects.IsNull() && !config.MembersRequireProjects.IsUnknown() {
		request.MembersRequireProjects = pointer(config.MembersRequireProjects.ValueBool())
	}
	if !config.MembersCreateProjects.IsNull() && !config.MembersCreateProjects.IsUnknown() {
		request.MembersCreateProjects = pointer(config.MembersCreateProjects.ValueBool())
	}
	if !config.AllowLocalRunners.IsNull() && !config.AllowLocalRunners.IsUnknown() {
		request.AllowLocalRunners = pointer(config.AllowLocalRunners.ValueBool())
	}
	if !config.MaximumRunningEnvironmentsPerUser.IsNull() && !config.MaximumRunningEnvironmentsPerUser.IsUnknown() {
		request.MaximumRunningEnvironmentsPerUser = pointer(config.MaximumRunningEnvironmentsPerUser.ValueInt64())
	}
	if !config.MaximumEnvironmentsPerUser.IsNull() && !config.MaximumEnvironmentsPerUser.IsUnknown() {
		request.MaximumEnvironmentsPerUser = pointer(config.MaximumEnvironmentsPerUser.ValueInt64())
	}
	if !config.PortSharingDisabled.IsNull() && !config.PortSharingDisabled.IsUnknown() {
		request.PortSharingDisabled = pointer(config.PortSharingDisabled.ValueBool())
	}

	if diags.HasError() {
		return nil, diags
	}
	return request, diags
}

func validateOrganizationPoliciesModel(config organizationPoliciesModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if !config.MaximumEnvironmentTimeout.IsNull() && !config.MaximumEnvironmentTimeout.IsUnknown() {
		_, durationDiags := parseMaximumEnvironmentTimeout(config.MaximumEnvironmentTimeout)
		diags.Append(durationDiags...)
	}

	requireProjects := config.MembersRequireProjects
	createProjects := config.MembersCreateProjects
	if !requireProjects.IsUnknown() && !createProjects.IsUnknown() {
		if requireProjects.IsNull() != createProjects.IsNull() {
			diags.AddAttributeError(
				path.Root("members_require_projects"),
				"Invalid Project Member Policy",
				"members_require_projects and members_create_projects must be configured together.",
			)
		} else if !requireProjects.IsNull() && requireProjects.ValueBool() == createProjects.ValueBool() {
			diags.AddAttributeError(
				path.Root("members_create_projects"),
				"Invalid Project Member Policy",
				"members_require_projects and members_create_projects must have opposite values.",
			)
		}
	}

	return diags
}

func parseMaximumEnvironmentTimeout(value types.String) (*durationpb.Duration, diag.Diagnostics) {
	var diags diag.Diagnostics
	duration, err := time.ParseDuration(value.ValueString())
	if err != nil {
		diags.AddAttributeError(
			path.Root("maximum_environment_timeout"),
			"Invalid Duration",
			"maximum_environment_timeout must be a Go duration such as \"30m\", \"2h\", or \"0s\".",
		)
		return nil, diags
	}
	if duration < 0 {
		diags.AddAttributeError(path.Root("maximum_environment_timeout"), "Invalid Duration", "maximum_environment_timeout must not be negative.")
		return nil, diags
	}
	if duration != 0 && duration < 30*time.Minute {
		diags.AddAttributeError(path.Root("maximum_environment_timeout"), "Invalid Duration", "maximum_environment_timeout must be 0s or at least 30m.")
		return nil, diags
	}
	return durationpb.New(duration), diags
}

func mapOrganizationPoliciesToModel(policies *v1.OrganizationPolicies, prior organizationPoliciesModel) organizationPoliciesModel {
	state := organizationPoliciesModel{
		ID:                                types.StringValue(policies.GetOrganizationId()),
		MembersRequireProjects:            types.BoolValue(policies.GetMembersRequireProjects()),
		MembersCreateProjects:             types.BoolValue(policies.GetMembersCreateProjects()),
		AllowLocalRunners:                 types.BoolValue(policies.GetAllowLocalRunners()),
		MaximumRunningEnvironmentsPerUser: types.Int64Value(policies.GetMaximumRunningEnvironmentsPerUser()),
		MaximumEnvironmentsPerUser:        types.Int64Value(policies.GetMaximumEnvironmentsPerUser()),
		PortSharingDisabled:               types.BoolValue(policies.GetPortSharingDisabled()),
	}

	if timeout := policies.GetMaximumEnvironmentTimeout(); timeout != nil {
		actual := timeout.AsDuration()
		preserved := false
		if !prior.MaximumEnvironmentTimeout.IsNull() && !prior.MaximumEnvironmentTimeout.IsUnknown() {
			if parsed, err := time.ParseDuration(prior.MaximumEnvironmentTimeout.ValueString()); err == nil && parsed == actual {
				state.MaximumEnvironmentTimeout = prior.MaximumEnvironmentTimeout
				preserved = true
			}
		}
		if !preserved {
			state.MaximumEnvironmentTimeout = types.StringValue(actual.String())
		}
	} else {
		state.MaximumEnvironmentTimeout = types.StringNull()
	}

	return state
}

func fallbackOrganizationPoliciesState(current *v1.OrganizationPolicies, update *v1.UpdateOrganizationPoliciesRequest, plan organizationPoliciesModel) organizationPoliciesModel {
	state := mapOrganizationPoliciesToModel(current, plan)
	if update.MaximumEnvironmentTimeout != nil {
		if configuredString(plan.MaximumEnvironmentTimeout) {
			state.MaximumEnvironmentTimeout = plan.MaximumEnvironmentTimeout
		} else {
			state.MaximumEnvironmentTimeout = types.StringValue(update.MaximumEnvironmentTimeout.AsDuration().String())
		}
	}
	if update.MembersRequireProjects != nil {
		state.MembersRequireProjects = types.BoolValue(update.GetMembersRequireProjects())
	}
	if update.MembersCreateProjects != nil {
		state.MembersCreateProjects = types.BoolValue(update.GetMembersCreateProjects())
	}
	if update.AllowLocalRunners != nil {
		state.AllowLocalRunners = types.BoolValue(update.GetAllowLocalRunners())
	}
	if update.MaximumRunningEnvironmentsPerUser != nil {
		state.MaximumRunningEnvironmentsPerUser = types.Int64Value(update.GetMaximumRunningEnvironmentsPerUser())
	}
	if update.MaximumEnvironmentsPerUser != nil {
		state.MaximumEnvironmentsPerUser = types.Int64Value(update.GetMaximumEnvironmentsPerUser())
	}
	if update.PortSharingDisabled != nil {
		state.PortSharingDisabled = types.BoolValue(update.GetPortSharingDisabled())
	}
	return state
}

func hasOrganizationPolicyUpdates(request *v1.UpdateOrganizationPoliciesRequest) bool {
	return request.MaximumEnvironmentTimeout != nil ||
		request.MembersRequireProjects != nil ||
		request.MembersCreateProjects != nil ||
		request.AllowLocalRunners != nil ||
		request.MaximumRunningEnvironmentsPerUser != nil ||
		request.MaximumEnvironmentsPerUser != nil ||
		request.PortSharingDisabled != nil
}

func configuredString(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func configuredBool(value types.Bool) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func configuredInt64(value types.Int64) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func setOrganizationPoliciesBaseline(ctx context.Context, state organizationPoliciesPrivateState, policies *v1.OrganizationPolicies, config organizationPoliciesModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if state == nil {
		diags.AddError("Unable to store organization policy baseline", "Terraform did not provide private state storage for ona_organization_policies.")
		return diags
	}
	if policies == nil {
		diags.AddError("Unable to store organization policy baseline", "The Ona API returned an empty organization policy object.")
		return diags
	}

	baseline := captureOrganizationPoliciesBaseline(&organizationPoliciesBaseline{}, policies, config)
	diags.Append(storeOrganizationPoliciesBaseline(ctx, state, baseline)...)
	return diags
}

func storeOrganizationPoliciesBaseline(ctx context.Context, state organizationPoliciesPrivateState, baseline *organizationPoliciesBaseline) diag.Diagnostics {
	var diags diag.Diagnostics
	if state == nil {
		diags.AddError("Unable to store organization policy baseline", "Terraform did not provide private state storage for ona_organization_policies.")
		return diags
	}

	data, err := json.Marshal(baseline)
	if err != nil {
		diags.AddError("Unable to store organization policy baseline", fmt.Sprintf("Could not encode the organization policy baseline: %s", err))
		return diags
	}
	diags.Append(state.SetKey(ctx, organizationPoliciesBaselinePrivateKey, data)...)
	return diags
}

func ensureOrganizationPoliciesBaseline(ctx context.Context, prior, next organizationPoliciesPrivateState, policies *v1.OrganizationPolicies) diag.Diagnostics {
	var diags diag.Diagnostics
	if prior != nil {
		data, stateDiags := prior.GetKey(ctx, organizationPoliciesBaselinePrivateKey)
		diags.Append(stateDiags...)
		if diags.HasError() {
			return diags
		}
		if len(data) > 0 {
			if next == nil {
				diags.AddError("Unable to store organization policy baseline", "Terraform did not provide private state storage for ona_organization_policies.")
				return diags
			}
			diags.Append(next.SetKey(ctx, organizationPoliciesBaselinePrivateKey, data)...)
			return diags
		}
	}
	return setOrganizationPoliciesBaseline(ctx, next, policies, organizationPoliciesModel{})
}

func mergeOrganizationPoliciesBaseline(ctx context.Context, prior, next organizationPoliciesPrivateState, policies *v1.OrganizationPolicies, config organizationPoliciesModel) diag.Diagnostics {
	var diags diag.Diagnostics
	baseline := &organizationPoliciesBaseline{}
	if prior != nil {
		data, stateDiags := prior.GetKey(ctx, organizationPoliciesBaselinePrivateKey)
		diags.Append(stateDiags...)
		if diags.HasError() {
			return diags
		}
		if len(data) > 0 {
			if err := json.Unmarshal(data, baseline); err != nil {
				diags.AddError("Unable to update organization policy baseline", fmt.Sprintf("Could not decode the organization policy baseline: %s", err))
				return diags
			}
		}
	}

	captureOrganizationPoliciesBaseline(baseline, policies, config)
	diags.Append(storeOrganizationPoliciesBaseline(ctx, next, baseline)...)
	return diags
}

func getOrganizationPoliciesBaseline(ctx context.Context, state organizationPoliciesPrivateState) (*organizationPoliciesBaseline, diag.Diagnostics) {
	var diags diag.Diagnostics
	if state == nil {
		diags.AddError("Unable to restore organization policy baseline", "Terraform did not provide private state storage for ona_organization_policies.")
		return nil, diags
	}

	data, stateDiags := state.GetKey(ctx, organizationPoliciesBaselinePrivateKey)
	diags.Append(stateDiags...)
	if diags.HasError() {
		return nil, diags
	}
	if len(data) == 0 {
		diags.AddError(
			"Unable to restore organization policy baseline",
			"The organization policy baseline is missing from private state, so the original policy values are unknown. "+
				"Remove the resource from state with \"terraform state rm\" and restore the intended policy values manually.",
		)
		return nil, diags
	}

	var baseline organizationPoliciesBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		diags.AddError("Unable to restore organization policy baseline", fmt.Sprintf("Could not decode the organization policy baseline: %s", err))
		return nil, diags
	}
	return &baseline, diags
}

func captureOrganizationPoliciesBaseline(baseline *organizationPoliciesBaseline, policies *v1.OrganizationPolicies, config organizationPoliciesModel) *organizationPoliciesBaseline {
	if baseline.MaximumEnvironmentTimeout == nil && configuredString(config.MaximumEnvironmentTimeout) {
		if timeout := policies.GetMaximumEnvironmentTimeout(); timeout != nil {
			baseline.MaximumEnvironmentTimeout = durationpb.New(timeout.AsDuration())
		} else {
			baseline.MaximumEnvironmentTimeout = durationpb.New(0)
		}
	}
	if baseline.MembersRequireProjects == nil && configuredBool(config.MembersRequireProjects) {
		baseline.MembersRequireProjects = pointer(policies.GetMembersRequireProjects())
	}
	if baseline.MembersCreateProjects == nil && configuredBool(config.MembersCreateProjects) {
		baseline.MembersCreateProjects = pointer(policies.GetMembersCreateProjects())
	}
	if baseline.AllowLocalRunners == nil && configuredBool(config.AllowLocalRunners) {
		baseline.AllowLocalRunners = pointer(policies.GetAllowLocalRunners())
	}
	if baseline.MaximumRunningEnvironmentsPerUser == nil && configuredInt64(config.MaximumRunningEnvironmentsPerUser) {
		baseline.MaximumRunningEnvironmentsPerUser = pointer(policies.GetMaximumRunningEnvironmentsPerUser())
	}
	if baseline.MaximumEnvironmentsPerUser == nil && configuredInt64(config.MaximumEnvironmentsPerUser) {
		baseline.MaximumEnvironmentsPerUser = pointer(policies.GetMaximumEnvironmentsPerUser())
	}
	if baseline.PortSharingDisabled == nil && configuredBool(config.PortSharingDisabled) {
		baseline.PortSharingDisabled = pointer(policies.GetPortSharingDisabled())
	}
	return baseline
}

func restoreOrganizationPoliciesRequest(organizationID string, baseline *organizationPoliciesBaseline) *v1.UpdateOrganizationPoliciesRequest {
	return &v1.UpdateOrganizationPoliciesRequest{
		OrganizationId:                    organizationID,
		MaximumEnvironmentTimeout:         baseline.MaximumEnvironmentTimeout,
		MembersRequireProjects:            baseline.MembersRequireProjects,
		MembersCreateProjects:             baseline.MembersCreateProjects,
		AllowLocalRunners:                 baseline.AllowLocalRunners,
		MaximumRunningEnvironmentsPerUser: baseline.MaximumRunningEnvironmentsPerUser,
		MaximumEnvironmentsPerUser:        baseline.MaximumEnvironmentsPerUser,
		PortSharingDisabled:               baseline.PortSharingDisabled,
	}
}

func guardOrganizationPoliciesID(stateID types.String, authenticatedOrganizationID string, diags *diag.Diagnostics) bool {
	if stateID.IsNull() || stateID.IsUnknown() || stateID.ValueString() == "" {
		diags.AddError("Missing organization policies ID", "The ona_organization_policies state does not contain an organization ID. Re-import the resource.")
		return false
	}
	if stateID.ValueString() != authenticatedOrganizationID {
		diags.AddError(
			"Organization policies belong to a different organization",
			fmt.Sprintf("State contains organization ID %q, but the provider token is authenticated for %q.", stateID.ValueString(), authenticatedOrganizationID),
		)
		return false
	}
	return true
}

func pointer[T any](value T) *T {
	return &value
}
