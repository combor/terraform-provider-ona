package provider

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
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
	_ resource.Resource                     = &groupMembershipResource{}
	_ resource.ResourceWithConfigValidators = &groupMembershipResource{}
	_ resource.ResourceWithImportState      = &groupMembershipResource{}
)

type groupMembershipResource struct {
	client *sdk.Client
}

func NewGroupMembershipResource() resource.Resource {
	return &groupMembershipResource{}
}

type groupMembershipModel struct {
	ID               types.String `tfsdk:"id"`
	GroupID          types.String `tfsdk:"group_id"`
	UserID           types.String `tfsdk:"user_id"`
	ServiceAccountID types.String `tfsdk:"service_account_id"`
}

func (r *groupMembershipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_membership"
}

func (r *groupMembershipResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages one user's or service account's membership in a custom group. Other memberships are left unchanged. Set exactly one of `user_id` or `service_account_id`. Custom groups require the Enterprise plan; membership changes require organization-admin or group-admin permissions.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Membership ID returned by the API.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"group_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group the member belongs to. Changing this value replaces the membership.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "User to add to the group. Set exactly one of `user_id` or `service_account_id`. Changing this value replaces the membership.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"service_account_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Service account to add to the group. Set exactly one of `service_account_id` or `user_id`. Changing this value replaces the membership.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *groupMembershipResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(path.MatchRoot("user_id"), path.MatchRoot("service_account_id")),
	}
}

func (r *groupMembershipResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.client = client
}

func (r *groupMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupMembershipModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API is not documented to reject an existing member. Adopting one
	// silently would let a later destroy remove a membership Terraform never
	// created.
	existing, err := r.getMembership(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read group membership before creation", err.Error())
		return
	}
	if existing != nil {
		resp.Diagnostics.AddError("Group membership already exists", "The member already belongs to the group. Import the membership to manage it with Terraform.")
		return
	}

	createResp, err := r.client.Services.Group.CreateMembership(ctx, connect.NewRequest(&v1.CreateMembershipRequest{
		GroupId: plan.GroupID.ValueString(),
		Subject: groupMembershipSubject(plan),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create group membership", err.Error())
		return
	}

	id := createResp.Msg.GetMember().GetId()
	if id == "" {
		// Failing a create taints the resource, so recover the ID now rather
		// than on a later refresh.
		member, err := r.getMembership(ctx, plan)
		if err != nil {
			resp.Diagnostics.AddError("Failed to read group membership after creation", err.Error())
		}
		id = member.GetId()
	}

	plan.ID = stringValueOrNull(id)
	// Save the lookup fields even without an ID so the new membership stays
	// tracked.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if plan.ID.IsNull() {
		resp.Diagnostics.AddError("Missing group membership ID", "The API returned no membership ID after creation. Terraform has tainted the resource and will replace it on the next apply; run `terraform untaint` first to keep the membership.")
	}
}

func (r *groupMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	member, err := r.getMembership(ctx, state)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read group membership", err.Error())
		return
	}
	if member == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Keep the stored ID when the API omits it, so the resource can still be
	// planned and destroyed.
	state.ID = mergeStringWithPrior(member.GetId(), state.ID)
	if state.ID.IsNull() {
		resp.Diagnostics.AddError("Missing group membership ID", "The API returned a membership without an ID.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupMembershipResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Group membership cannot be updated", "Changing the group or member requires replacing the membership.")
}

func (r *groupMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve the ID by member instead of trusting state: it is absent after
	// an import or partial create, and stale once the membership has been
	// re-added outside Terraform.
	member, err := r.getMembership(ctx, state)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read group membership before deletion", err.Error())
		return
	}
	if member == nil {
		return
	}
	id := mergeStringWithPrior(member.GetId(), state.ID)
	if id.ValueString() == "" {
		resp.Diagnostics.AddError("Missing group membership ID", "The API returned a membership without an ID.")
		return
	}

	_, err = r.client.Services.Group.DeleteMembership(ctx, connect.NewRequest(&v1.DeleteMembershipRequest{
		MembershipId: id.ValueString(),
	}))
	if err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete group membership", err.Error())
	}
}

func (r *groupMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 || parts[0] == "" || parts[2] == "" || (parts[1] != "user" && parts[1] != "service_account") {
		resp.Diagnostics.AddError("Invalid import ID", "Expected <group-id>/user/<user-id> or <group-id>/service_account/<service-account-id>.")
		return
	}

	state := groupMembershipModel{GroupID: types.StringValue(parts[0])}
	if parts[1] == "user" {
		state.UserID = types.StringValue(parts[2])
	} else {
		state.ServiceAccountID = types.StringValue(parts[2])
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// getMembership looks a membership up by group and member. It returns nil when
// the member is not in the group.
func (r *groupMembershipResource) getMembership(ctx context.Context, model groupMembershipModel) (*v1.GroupMembership, error) {
	getResp, err := r.client.Services.Group.GetMembership(ctx, connect.NewRequest(&v1.GetMembershipRequest{
		GroupId: model.GroupID.ValueString(),
		Subject: groupMembershipSubject(model),
	}))
	if isAPINotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return getResp.Msg.GetMember(), nil
}

func groupMembershipSubject(model groupMembershipModel) *v1.Subject {
	if !model.UserID.IsNull() {
		return &v1.Subject{Id: model.UserID.ValueString(), Principal: v1.Principal_PRINCIPAL_USER}
	}
	return &v1.Subject{Id: model.ServiceAccountID.ValueString(), Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT}
}
