package provider

import (
	"context"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &authenticatedIdentityDataSource{}

type authenticatedIdentityDataSource struct {
	client *sdk.Client
}

func NewAuthenticatedIdentityDataSource() datasource.DataSource {
	return &authenticatedIdentityDataSource{}
}

func (d *authenticatedIdentityDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_authenticated_identity"
}

func (d *authenticatedIdentityDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up the currently authenticated Gitpod identity.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Authenticated subject ID.",
			},
			"principal": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Authenticated subject principal.",
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization ID associated with the authenticated identity.",
			},
			"organization_tier": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization tier (e.g. plan level) of the authenticated identity.",
			},
		},
	}
}

func (d *authenticatedIdentityDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	d.client = client
}

func (d *authenticatedIdentityDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	getResp, err := d.client.Services.Identity.GetAuthenticatedIdentity(ctx, connect.NewRequest(&v1.GetAuthenticatedIdentityRequest{}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read authenticated identity", err.Error())
		return
	}

	state := mapAuthenticatedIdentityToDataSourceModel(getResp.Msg)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func mapAuthenticatedIdentityToDataSourceModel(resp *v1.GetAuthenticatedIdentityResponse) authenticatedIdentityDataSourceModel {
	return authenticatedIdentityDataSourceModel{
		ID:               stringValueOrNull(resp.GetSubject().GetId()),
		Principal:        stringValueOrNull(enumString(resp.GetSubject().GetPrincipal())),
		OrganizationID:   stringValueOrNull(resp.GetOrganizationId()),
		OrganizationTier: stringValueOrNull(resp.GetOrganizationTier()),
	}
}

type authenticatedIdentityDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	Principal        types.String `tfsdk:"principal"`
	OrganizationID   types.String `tfsdk:"organization_id"`
	OrganizationTier types.String `tfsdk:"organization_tier"`
}
