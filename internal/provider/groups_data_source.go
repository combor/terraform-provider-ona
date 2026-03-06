package provider

import (
	"context"
	"fmt"
	"sort"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &groupsDataSource{}

type groupsDataSource struct {
	client *gitpod.Client
}

func NewGroupsDataSource() datasource.DataSource {
	return &groupsDataSource{}
}

func (d *groupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_groups"
}

func (d *groupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List Gitpod groups in the organization.",
		Attributes: map[string]schema.Attribute{
			"groups": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Groups in the organization.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
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
				},
			},
		},
		Blocks: map[string]schema.Block{
			"filter": schema.ListNestedBlock{
				MarkdownDescription: "Filter groups. Supported filter names: `name`.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Name of the field to filter by.",
						},
						"values": schema.ListAttribute{
							ElementType:         types.StringType,
							Required:            true,
							MarkdownDescription: "Values to match against. A group matches if the field equals any of the values.",
						},
					},
				},
			},
		},
	}
}

func (d *groupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*gitpod.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("Expected *gitpod.Client, got %T", req.ProviderData))
		return
	}

	d.client = client
}

func (d *groupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config groupsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, f := range config.Filters {
		if f.Name.ValueString() != "name" {
			resp.Diagnostics.AddError("Unsupported filter",
				fmt.Sprintf("Filter %q is not supported. Supported filters: name", f.Name.ValueString()))
			return
		}
	}

	iter := d.client.Groups.ListAutoPaging(ctx, gitpod.GroupListParams{
		Pagination: gitpod.F(gitpod.GroupListParamsPagination{
			PageSize: gitpod.F(int64(100)),
		}),
	})

	groups := make([]gitpod.Group, 0)
	for iter.Next() {
		group := iter.Current()
		if matchesGroupFilters(group, config.Filters) {
			groups = append(groups, group)
		}
	}
	if err := iter.Err(); err != nil {
		resp.Diagnostics.AddError("Failed to list groups", err.Error())
		return
	}

	state := mapGroupsToDataSourceModel(config.Filters, groups)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func matchesGroupFilters(group gitpod.Group, filters []groupsFilterModel) bool {
	for _, f := range filters {
		var fieldValue string
		switch f.Name.ValueString() {
		case "name":
			fieldValue = group.Name
		default:
			return false
		}

		matched := false
		for _, v := range f.Values {
			if v.ValueString() == fieldValue {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func mapGroupsToDataSourceModel(filters []groupsFilterModel, groups []gitpod.Group) groupsDataSourceModel {
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID < groups[j].ID
	})

	state := groupsDataSourceModel{
		Filters: filters,
		Groups:  make([]groupDataSourceModel, 0, len(groups)),
	}

	for _, group := range groups {
		state.Groups = append(state.Groups, mapGroupToDataSourceModel(group))
	}

	return state
}

type groupsDataSourceModel struct {
	Filters []groupsFilterModel    `tfsdk:"filter"`
	Groups  []groupDataSourceModel `tfsdk:"groups"`
}

type groupsFilterModel struct {
	Name   types.String   `tfsdk:"name"`
	Values []types.String `tfsdk:"values"`
}
