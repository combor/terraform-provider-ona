package provider

import (
	"context"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &projectsDataSource{}

type projectsDataSource struct {
	client *sdk.Client
}

func NewProjectsDataSource() datasource.DataSource {
	return &projectsDataSource{}
}

func (d *projectsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_projects"
}

func (d *projectsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	projectAttributes := projectDataSourceAttributes()
	projectAttributes["id"] = schema.StringAttribute{
		Computed:            true,
		MarkdownDescription: "Project ID.",
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "List Gitpod projects in the organization. Filters are combined, so a project must match all of the ones that are set; list filters match a project when any of their values does.",
		Attributes: map[string]schema.Attribute{
			"search": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Case-insensitive search across project name, project ID, and repository name.",
				Validators:          []validator.String{stringvalidator.UTF8LengthAtMost(256)},
			},
			"project_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Only return projects with these IDs.",
				Validators:          []validator.List{listvalidator.SizeAtMost(25)},
			},
			"runner_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Only return projects that use environment classes from these runners.",
				Validators:          []validator.List{listvalidator.SizeAtMost(25)},
			},
			"runner_kinds": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Only return projects that use environment classes from runners of these kinds, such as `RUNNER_KIND_REMOTE`.",
				Validators: []validator.List{
					listvalidator.SizeAtMost(25),
					listvalidator.ValueStringsAre(enumValidators(v1.RunnerKind_value)...),
				},
			},
			"remote_uris": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Only return projects whose initializer git `remote_uri` exactly matches one of these URIs.",
				Validators:          []validator.List{listvalidator.SizeAtMost(25)},
			},
			"creator_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Only return projects created by these subjects.",
				Validators:          []validator.List{listvalidator.SizeAtMost(25)},
			},
			"sort": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Order of `projects`. Defaults to `id` ascending, which is creation order and stays stable between runs. `popularity` ranks projects by recent environment creation activity and is recomputed periodically, so its order can change between runs.",
				Attributes: map[string]schema.Attribute{
					"field": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Field to sort by: `id` or `popularity`.",
						Validators:          []validator.String{stringvalidator.OneOf("id", "popularity")},
					},
					"order": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Sort order: `SORT_ORDER_ASC` or `SORT_ORDER_DESC`.",
						Validators:          enumValidators(v1.SortOrder_value),
					},
				},
			},
			"limit": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Maximum number of projects to return. Pages stop being fetched once this many projects are collected. By default all matching projects are returned.",
				Validators:          []validator.Int64{int64validator.AtLeast(1)},
			},
			"include_count": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether to request a bounded total count of matching projects in `total_count`.",
			},
			"projects": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching projects, in `sort` order.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: projectAttributes,
				},
			},
			"total_count": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Total number of matching projects, regardless of `limit`. Set only when `include_count` is true. The API bounds this count, so check `total_count_relation` to see whether it is exact.",
			},
			"total_count_relation": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "`COUNT_RESPONSE_RELATION_EQ` when `total_count` is exact, or `COUNT_RESPONSE_RELATION_GTE` when there are at least `total_count` matching projects.",
			},
		},
	}
}

func (d *projectsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	d.client = client
}

func (d *projectsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config projectsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	filter := buildListProjectsFilter(config, &resp.Diagnostics)
	sort := buildListProjectsSort(config.Sort, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	limited := !config.Limit.IsNull()
	remaining := config.Limit.ValueInt64()
	var count *v1.CountResponse
	projects, err := collectPaged(func(token string) ([]*v1.Project, string, error) {
		listReq := &v1.ListProjectsRequest{
			Pagination: &v1.PaginationRequest{PageSize: 100, Token: token},
			Filter:     filter,
			Sort:       sort,
		}
		if limited && remaining < 100 {
			listReq.Pagination.PageSize = int32(remaining)
		}
		// The API only returns the count on the first page.
		if token == "" && config.IncludeCount.ValueBool() {
			listReq.Count = &v1.CountRequest{Include: true}
		}

		listResp, err := d.client.Services.Project.ListProjects(ctx, connect.NewRequest(listReq))
		if err != nil {
			return nil, "", err
		}
		if token == "" {
			count = listResp.Msg.GetCount()
		}

		page := listResp.Msg.GetProjects()
		next := listResp.Msg.GetPagination().GetNextToken()
		if limited {
			if int64(len(page)) >= remaining {
				page = page[:remaining]
				next = ""
			}
			remaining -= int64(len(page))
		}
		return page, next, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to list projects", err.Error())
		return
	}

	state := config
	state.Projects = make([]projectModel, 0, len(projects))
	for _, project := range projects {
		model, diags := mapProjectToDataSourceModel(ctx, project)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Projects = append(state.Projects, model)
	}

	state.TotalCount = types.Int64Null()
	if count != nil {
		state.TotalCount = types.Int64Value(int64(count.GetValue()))
	}
	state.TotalCountRelation = stringValueOrNull(enumString(count.GetRelation()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func buildListProjectsFilter(config projectsDataSourceModel, diagnostics *diag.Diagnostics) *v1.ListProjectsRequest_Filter {
	runnerKinds := make([]v1.RunnerKind, 0, len(config.RunnerKinds))
	for _, kind := range config.RunnerKinds {
		runnerKinds = append(runnerKinds, enumValue[v1.RunnerKind]("runner_kinds", kind.ValueString(), v1.RunnerKind_value, diagnostics))
	}

	return &v1.ListProjectsRequest_Filter{
		ProjectIds:     stringValues(config.ProjectIDs),
		Search:         config.Search.ValueString(),
		RunnerIds:      stringValues(config.RunnerIDs),
		RunnerKinds:    runnerKinds,
		SpecRemoteUris: stringValues(config.RemoteURIs),
		CreatorIds:     stringValues(config.CreatorIDs),
	}
}

// buildListProjectsSort defaults to id ascending instead of the API's
// popularity order, which is recomputed in the background and so can reorder
// results between runs.
func buildListProjectsSort(config *projectsSortModel, diagnostics *diag.Diagnostics) *v1.Sort {
	if config == nil {
		return &v1.Sort{Field: "id", Order: v1.SortOrder_SORT_ORDER_ASC}
	}

	return &v1.Sort{
		Field: config.Field.ValueString(),
		Order: enumValue[v1.SortOrder]("sort.order", config.Order.ValueString(), v1.SortOrder_value, diagnostics),
	}
}

func stringValues(values []types.String) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.ValueString())
	}
	return result
}

type projectsDataSourceModel struct {
	Search             types.String       `tfsdk:"search"`
	ProjectIDs         []types.String     `tfsdk:"project_ids"`
	RunnerIDs          []types.String     `tfsdk:"runner_ids"`
	RunnerKinds        []types.String     `tfsdk:"runner_kinds"`
	RemoteURIs         []types.String     `tfsdk:"remote_uris"`
	CreatorIDs         []types.String     `tfsdk:"creator_ids"`
	Sort               *projectsSortModel `tfsdk:"sort"`
	Limit              types.Int64        `tfsdk:"limit"`
	IncludeCount       types.Bool         `tfsdk:"include_count"`
	Projects           []projectModel     `tfsdk:"projects"`
	TotalCount         types.Int64        `tfsdk:"total_count"`
	TotalCountRelation types.String       `tfsdk:"total_count_relation"`
}

type projectsSortModel struct {
	Field types.String `tfsdk:"field"`
	Order types.String `tfsdk:"order"`
}
