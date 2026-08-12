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
	_ resource.Resource                = &projectPolicyResource{}
	_ resource.ResourceWithImportState = &projectPolicyResource{}
)

type projectPolicyResource struct {
	client *sdk.Client
}

func NewProjectPolicyResource() resource.Resource {
	return &projectPolicyResource{}
}

type projectPolicyModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	GroupID   types.String `tfsdk:"group_id"`
	Role      types.String `tfsdk:"role"`
}

func (r *projectPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_policy"
}

func (r *projectPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Grants a group a role on a project. One resource per project/group pair. Members also need access to a runner the project uses before they can start environments; see `ona_runner_policy`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Policy identifier in the form `<project-id>/<group-id>`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project the group is granted access to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"group_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group the role is granted to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"role": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Role granted to the group: `PROJECT_ROLE_ADMIN`, `PROJECT_ROLE_EDITOR` or `PROJECT_ROLE_USER`.",
				Validators:          enumValidators(v1.ProjectRole_value),
			},
		},
	}
}

func (r *projectPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.client = client
}

func (r *projectPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := projectRoleValue(plan.Role, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := r.client.Services.Project.CreateProjectPolicy(ctx, connect.NewRequest(&v1.CreateProjectPolicyRequest{
		ProjectId: plan.ProjectID.ValueString(),
		GroupId:   plan.GroupID.ValueString(),
		Role:      role,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create project policy", err.Error())
		return
	}

	state := mapProjectPolicyToModel(plan.ProjectID.ValueString(), createResp.Msg.GetPolicy())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	policy, err := r.findProjectPolicy(ctx, projectID, state.GroupID.ValueString())
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read project policy", err.Error())
		return
	}
	if policy == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	newState := mapProjectPolicyToModel(projectID, policy)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *projectPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := projectRoleValue(plan.Role, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	updateResp, err := r.client.Services.Project.UpdateProjectPolicy(ctx, connect.NewRequest(&v1.UpdateProjectPolicyRequest{
		ProjectId: plan.ProjectID.ValueString(),
		GroupId:   plan.GroupID.ValueString(),
		Role:      role,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update project policy", err.Error())
		return
	}

	state := mapProjectPolicyToModel(plan.ProjectID.ValueString(), updateResp.Msg.GetPolicy())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Services.Project.DeleteProjectPolicy(ctx, connect.NewRequest(&v1.DeleteProjectPolicyRequest{
		ProjectId: state.ProjectID.ValueString(),
		GroupId:   state.GroupID.ValueString(),
	}))
	if err != nil {
		if isAPINotFound(err) {
			return
		}

		resp.Diagnostics.AddError("Failed to delete project policy", err.Error())
	}
}

func (r *projectPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectID, groupID, err := parsePairImportID(req.ID, "<project-id>/<group-id>")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), pairID(projectID, groupID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group_id"), groupID)...)
}

func (r *projectPolicyResource) findProjectPolicy(ctx context.Context, projectID, groupID string) (*v1.ProjectPolicy, error) {
	token := ""
	for {
		listResp, err := r.client.Services.Project.ListProjectPolicies(ctx, connect.NewRequest(&v1.ListProjectPoliciesRequest{
			ProjectId:  projectID,
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

func projectRoleValue(role types.String, diagnostics *diag.Diagnostics) v1.ProjectRole {
	return enumValue[v1.ProjectRole]("role", role.ValueString(), v1.ProjectRole_value, diagnostics)
}

func mapProjectPolicyToModel(projectID string, policy *v1.ProjectPolicy) projectPolicyModel {
	groupID := policy.GetGroupId()

	return projectPolicyModel{
		ID:        types.StringValue(pairID(projectID, groupID)),
		ProjectID: types.StringValue(projectID),
		GroupID:   types.StringValue(groupID),
		Role:      stringValueOrNull(enumString(policy.GetRole())),
	}
}
