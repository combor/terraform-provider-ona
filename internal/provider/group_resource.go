package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &groupResource{}
	_ resource.ResourceWithImportState = &groupResource{}
)

type groupResource struct {
	client *sdk.Client
}

func NewGroupResource() resource.Resource {
	return &groupResource{}
}

type groupModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	OrganizationID types.String `tfsdk:"organization_id"`
	MemberCount    types.Int64  `tfsdk:"member_count"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom group in the organization the API key authenticates to. System-managed groups cannot be managed with this resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Group ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group name. The API accepts 3 to 80 characters.",
				Validators:          []validator.String{stringvalidator.UTF8LengthBetween(3, 80)},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Group description. The API accepts at most 255 characters.",
				Validators:          []validator.String{stringvalidator.UTF8LengthAtMost(255)},
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization ID the group belongs to. Resolved from the authenticated identity.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			// member_count is deliberately left to go unknown on update: memberships
			// change outside Terraform, so pinning the prior value would risk an
			// inconsistent-result-after-apply error.
			"member_count": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of members in the group.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the group was created (RFC3339).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the group was last updated (RFC3339).",
			},
		},
	}
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.client = client
}

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationID, err := authenticatedOrganizationID(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to resolve organization", err.Error())
		return
	}

	createResp, err := r.client.Services.Group.CreateGroup(ctx, connect.NewRequest(&v1.CreateGroupRequest{
		OrganizationId: organizationID,
		Name:           plan.Name.ValueString(),
		Description:    plan.Description.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create group", err.Error())
		return
	}

	state := mapGroupToResourceModel(createResp.Msg.GetGroup(), plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Services.Group.GetGroup(ctx, connect.NewRequest(&v1.GetGroupRequest{
		Group: &v1.GetGroupRequest_Id{Id: state.ID.ValueString()},
	}))
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read group", err.Error())
		return
	}

	group := getResp.Msg.GetGroup()
	if group.GetSystemManaged() {
		resp.Diagnostics.AddError("Group is system-managed",
			fmt.Sprintf("Group %s is managed by Gitpod and cannot be changed or deleted through Terraform. Remove it from state with `terraform state rm`.", state.ID.ValueString()))
		return
	}

	newState := mapGroupToResourceModel(group, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var prior groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateResp, err := r.client.Services.Group.UpdateGroup(ctx, connect.NewRequest(&v1.UpdateGroupRequest{
		GroupId:     prior.ID.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update group", err.Error())
		return
	}

	state := mapGroupToResourceModel(updateResp.Msg.GetGroup(), plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Services.Group.DeleteGroup(ctx, connect.NewRequest(&v1.DeleteGroupRequest{
		GroupId: state.ID.ValueString(),
	}))
	if err != nil {
		if isAPINotFound(err) {
			return
		}

		resp.Diagnostics.AddError("Failed to delete group", err.Error())
	}
}

func (r *groupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapGroupToResourceModel converts an API group into resource state. The name
// and description come back from the API, but an explicitly empty description
// is preserved from the prior value so configuring "" does not read back as
// null and produce a perpetual diff.
func mapGroupToResourceModel(group *v1.Group, prior groupModel) groupModel {
	return groupModel{
		ID:             types.StringValue(group.GetId()),
		Name:           stringValueOrNull(group.GetName()),
		Description:    stringValueOrPriorExplicitEmpty(group.GetDescription(), prior.Description),
		OrganizationID: stringValueOrNull(group.GetOrganizationId()),
		MemberCount:    types.Int64Value(int64(group.GetMemberCount())),
		CreatedAt:      timeValueOrNull(group.GetCreatedAt()),
		UpdatedAt:      timeValueOrNull(group.GetUpdatedAt()),
	}
}
