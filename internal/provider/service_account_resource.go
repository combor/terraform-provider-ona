package provider

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	_ resource.Resource                = &serviceAccountResource{}
	_ resource.ResourceWithImportState = &serviceAccountResource{}
)

type serviceAccountResource struct {
	client *sdk.Client
}

func NewServiceAccountResource() resource.Resource {
	return &serviceAccountResource{}
}

type serviceAccountModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Description      types.String `tfsdk:"description"`
	ValidUntil       types.String `tfsdk:"valid_until"`
	OrganizationID   types.String `tfsdk:"organization_id"`
	CreatorID        types.String `tfsdk:"creator_id"`
	CreatorPrincipal types.String `tfsdk:"creator_principal"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

func (r *serviceAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (r *serviceAccountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a service account, the non-human identity automation authenticates as. Tokens are not managed by this resource; create them in the Ona UI or through the API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Service account ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Service account name. The API accepts 1 to 64 characters.",
				Validators:          []validator.String{stringvalidator.UTF8LengthBetween(1, 64)},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Service account description. The API accepts at most 500 characters.",
				Validators:          []validator.String{stringvalidator.UTF8LengthAtMost(500)},
			},
			"valid_until": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Expiry timestamp (RFC3339). Authentication fails after it passes. The API requires an expiry. Immutable: changing it replaces the service account.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization ID the service account belongs to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"creator_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the subject that created the service account.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"creator_principal": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Principal type of the creator.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the service account was created (RFC3339).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *serviceAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.client = client
}

func (r *serviceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validUntil := parseValidUntil(plan.ValidUntil, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := r.client.Services.ServiceAccount.CreateServiceAccount(ctx, connect.NewRequest(&v1.CreateServiceAccountRequest{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		ValidUntil:  validUntil,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create service account", err.Error())
		return
	}

	state := mapServiceAccountToModel(createResp.Msg.GetServiceAccount(), plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serviceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Services.ServiceAccount.GetServiceAccount(ctx, connect.NewRequest(&v1.GetServiceAccountRequest{
		ServiceAccountId: state.ID.ValueString(),
	}))
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read service account", err.Error())
		return
	}

	account := getResp.Msg.GetServiceAccount()
	if account.GetSuspended() {
		resp.State.RemoveResource(ctx)
		return
	}
	if account.GetSystemManaged() {
		resp.Diagnostics.AddError("Service account is system-managed",
			fmt.Sprintf("Service account %s is managed by Gitpod and cannot be changed or deleted through Terraform. Remove it from state with `terraform state rm`.", state.ID.ValueString()))
		return
	}

	newState := mapServiceAccountToModel(account, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *serviceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var prior serviceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateResp, err := r.client.Services.ServiceAccount.UpdateServiceAccount(ctx, connect.NewRequest(&v1.UpdateServiceAccountRequest{
		ServiceAccountId: prior.ID.ValueString(),
		Name:             pointer(plan.Name.ValueString()),
		Description:      pointer(plan.Description.ValueString()),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update service account", err.Error())
		return
	}

	state := mapServiceAccountToModel(updateResp.Msg.GetServiceAccount(), plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serviceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Services.ServiceAccount.DeleteServiceAccount(ctx, connect.NewRequest(&v1.DeleteServiceAccountRequest{
		ServiceAccountId: state.ID.ValueString(),
	}))
	if err != nil {
		if isAPINotFound(err) {
			return
		}

		resp.Diagnostics.AddError("Failed to delete service account", err.Error())
	}
}

func (r *serviceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func parseValidUntil(validUntil types.String, diagnostics *diag.Diagnostics) *timestamppb.Timestamp {
	if validUntil.IsNull() || validUntil.IsUnknown() {
		return nil
	}

	parsed, err := time.Parse(time.RFC3339, validUntil.ValueString())
	if err != nil {
		diagnostics.AddError("Invalid valid_until",
			fmt.Sprintf("valid_until must be an RFC3339 timestamp, for example 2027-01-31T00:00:00Z: %s", err.Error()))
		return nil
	}

	return timestamppb.New(parsed)
}

func timestampValueWithPrior(value *timestamppb.Timestamp, prior types.String) types.String {
	if !prior.IsNull() && !prior.IsUnknown() && value != nil {
		if configured, err := time.Parse(time.RFC3339, prior.ValueString()); err == nil && configured.Equal(value.AsTime()) {
			return prior
		}
	}

	return timeValueOrNull(value)
}

func mapServiceAccountToModel(account *v1.ServiceAccount, prior serviceAccountModel) serviceAccountModel {
	return serviceAccountModel{
		ID:               types.StringValue(account.GetId()),
		Name:             stringValueOrNull(account.GetName()),
		Description:      stringValueOrPriorExplicitEmpty(account.GetDescription(), prior.Description),
		ValidUntil:       timestampValueWithPrior(account.GetValidUntil(), prior.ValidUntil),
		OrganizationID:   stringValueOrNull(account.GetOrganizationId()),
		CreatorID:        stringValueOrNull(account.GetCreator().GetId()),
		CreatorPrincipal: stringValueOrNull(enumString(account.GetCreator().GetPrincipal())),
		CreatedAt:        timeValueOrNull(account.GetCreatedAt()),
	}
}
