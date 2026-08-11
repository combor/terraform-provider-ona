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
	_ resource.Resource                = &runnerPolicyResource{}
	_ resource.ResourceWithImportState = &runnerPolicyResource{}
)

type runnerPolicyResource struct {
	client *sdk.Client
}

func NewRunnerPolicyResource() resource.Resource {
	return &runnerPolicyResource{}
}

type runnerPolicyModel struct {
	ID       types.String `tfsdk:"id"`
	RunnerID types.String `tfsdk:"runner_id"`
	GroupID  types.String `tfsdk:"group_id"`
	Role     types.String `tfsdk:"role"`
}

func (r *runnerPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner_policy"
}

func (r *runnerPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Grants a group a role on a runner. One resource per runner/group pair.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Policy identifier in the form `<runner-id>/<group-id>`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"runner_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner the group is granted access to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"group_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group the role is granted to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"role": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Role granted to the group: `RUNNER_ROLE_ADMIN` or `RUNNER_ROLE_USER`.",
			},
		},
	}
}

func (r *runnerPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.client = client
}

func (r *runnerPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan runnerPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := runnerRoleValue(plan.Role, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := r.client.Services.Runner.CreateRunnerPolicy(ctx, connect.NewRequest(&v1.CreateRunnerPolicyRequest{
		RunnerId: plan.RunnerID.ValueString(),
		GroupId:  plan.GroupID.ValueString(),
		Role:     role,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create runner policy", err.Error())
		return
	}

	state := mapRunnerPolicyToModel(plan.RunnerID.ValueString(), createResp.Msg.GetPolicy())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *runnerPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state runnerPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	runnerID := state.RunnerID.ValueString()
	policy, err := r.findRunnerPolicy(ctx, runnerID, state.GroupID.ValueString())
	if err != nil {
		// A deleted runner takes its policies with it.
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read runner policy", err.Error())
		return
	}
	if policy == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	newState := mapRunnerPolicyToModel(runnerID, policy)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *runnerPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan runnerPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := runnerRoleValue(plan.Role, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	updateResp, err := r.client.Services.Runner.UpdateRunnerPolicy(ctx, connect.NewRequest(&v1.UpdateRunnerPolicyRequest{
		RunnerId: plan.RunnerID.ValueString(),
		GroupId:  plan.GroupID.ValueString(),
		Role:     role,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update runner policy", err.Error())
		return
	}

	state := mapRunnerPolicyToModel(plan.RunnerID.ValueString(), updateResp.Msg.GetPolicy())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *runnerPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state runnerPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Services.Runner.DeleteRunnerPolicy(ctx, connect.NewRequest(&v1.DeleteRunnerPolicyRequest{
		RunnerId: state.RunnerID.ValueString(),
		GroupId:  state.GroupID.ValueString(),
	}))
	if err != nil {
		if isAPINotFound(err) {
			return
		}

		resp.Diagnostics.AddError("Failed to delete runner policy", err.Error())
	}
}

func (r *runnerPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	runnerID, groupID, err := parsePairImportID(req.ID, "<runner-id>/<group-id>")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), pairID(runnerID, groupID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("runner_id"), runnerID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group_id"), groupID)...)
}

// findRunnerPolicy returns the runner's policy for the given group, or nil when
// the group has no policy on that runner. It stops at the first match rather
// than draining every page, so a later page failing cannot fail a lookup that
// already found its policy.
func (r *runnerPolicyResource) findRunnerPolicy(ctx context.Context, runnerID, groupID string) (*v1.RunnerPolicy, error) {
	token := ""
	for {
		listResp, err := r.client.Services.Runner.ListRunnerPolicies(ctx, connect.NewRequest(&v1.ListRunnerPoliciesRequest{
			RunnerId:   runnerID,
			Pagination: &v1.PaginationRequest{PageSize: 100, Token: token},
		}))
		if err != nil {
			return nil, err
		}

		for _, policy := range listResp.Msg.GetPolicies() {
			if policy.GetGroupId() == groupID {
				return policy, nil
			}
		}

		token = listResp.Msg.GetPagination().GetNextToken()
		if token == "" {
			return nil, nil
		}
	}
}

func runnerRoleValue(role types.String, diagnostics *diag.Diagnostics) v1.RunnerRole {
	return enumValue[v1.RunnerRole]("role", role.ValueString(), v1.RunnerRole_value, diagnostics)
}

func mapRunnerPolicyToModel(runnerID string, policy *v1.RunnerPolicy) runnerPolicyModel {
	groupID := policy.GetGroupId()

	return runnerPolicyModel{
		ID:       types.StringValue(pairID(runnerID, groupID)),
		RunnerID: types.StringValue(runnerID),
		GroupID:  types.StringValue(groupID),
		Role:     stringValueOrNull(enumString(policy.GetRole())),
	}
}
