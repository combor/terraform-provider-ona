package provider

import (
	"context"
	"fmt"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &groupDataSource{}

type groupDataSource struct {
	client *gitpod.Client
}

func NewGroupDataSource() datasource.DataSource {
	return &groupDataSource{}
}

func (d *groupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (d *groupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing Gitpod group by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group ID.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Group name.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Group description.",
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization ID the group belongs to.",
			},
			"member_count": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of members in the group.",
			},
			"direct_share": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the group uses direct sharing.",
			},
			"system_managed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the group is system-managed.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the group was created.",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the group was last updated.",
			},
		},
	}
}

func (d *groupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	d.client = client
}

func (d *groupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config groupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Groups.Get(ctx, gitpod.GroupGetParams{
		GroupID: gitpod.F(config.ID.ValueString()),
	})
	if err != nil {
		if isAPINotFound(err) {
			resp.Diagnostics.AddError("Group not found",
				fmt.Sprintf("No group found with ID %s", config.ID.ValueString()))
			return
		}

		resp.Diagnostics.AddError("Failed to read group", err.Error())
		return
	}

	state := mapGroupToDataSourceModel(getResp.Group)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func mapGroupToDataSourceModel(group gitpod.Group) groupDataSourceModel {
	return groupDataSourceModel{
		ID:             types.StringValue(group.ID),
		Name:           stringValueOrNull(group.Name),
		Description:    stringValueOrNull(group.Description),
		OrganizationID: stringValueOrNull(group.OrganizationID),
		MemberCount:    types.Int64Value(group.MemberCount),
		DirectShare:    types.BoolValue(group.DirectShare),
		SystemManaged:  types.BoolValue(group.SystemManaged),
		CreatedAt:      timeValueOrNull(group.CreatedAt),
		UpdatedAt:      timeValueOrNull(group.UpdatedAt),
	}
}

type groupDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	OrganizationID types.String `tfsdk:"organization_id"`
	MemberCount    types.Int64  `tfsdk:"member_count"`
	DirectShare    types.Bool   `tfsdk:"direct_share"`
	SystemManaged  types.Bool   `tfsdk:"system_managed"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}
