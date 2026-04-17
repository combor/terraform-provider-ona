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

var _ datasource.DataSource = &runnersDataSource{}

type runnersDataSource struct {
	client *gitpod.Client
}

func NewRunnersDataSource() datasource.DataSource {
	return &runnersDataSource{}
}

func (d *runnersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runners"
}

func (d *runnersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List Gitpod runners in the organization.",
		Attributes: map[string]schema.Attribute{
			"runners": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Runners in the organization.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Runner ID.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Human-readable runner name.",
						},
						"provider_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Runner provider type.",
						},
						"runner_manager_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Runner manager ID.",
						},
						"status": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"phase":   schema.StringAttribute{Computed: true},
								"message": schema.StringAttribute{Computed: true},
								"version": schema.StringAttribute{Computed: true},
								"region":  schema.StringAttribute{Computed: true},
							},
						},
					},
				},
			},
		},
		Blocks: map[string]schema.Block{
			"filter": schema.ListNestedBlock{
				MarkdownDescription: "Filter runners. Supported filter names: `name`, `runner_manager_id`.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Name of the field to filter by.",
						},
						"values": schema.ListAttribute{
							ElementType:         types.StringType,
							Required:            true,
							MarkdownDescription: "Values to match against. A runner matches if the field equals any of the values.",
						},
					},
				},
			},
		},
	}
}

func (d *runnersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	d.client = client
}

func (d *runnersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config runnersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, f := range config.Filters {
		switch f.Name.ValueString() {
		case "name", "runner_manager_id":
		default:
			resp.Diagnostics.AddError("Unsupported filter",
				fmt.Sprintf("Filter %q is not supported. Supported filters: name, runner_manager_id", f.Name.ValueString()))
			return
		}
	}

	iter := d.client.Runners.ListAutoPaging(ctx, gitpod.RunnerListParams{
		Pagination: gitpod.F(gitpod.RunnerListParamsPagination{
			PageSize: gitpod.F(int64(100)),
		}),
	})

	runners := make([]gitpod.Runner, 0)
	for iter.Next() {
		runner := iter.Current()
		if matchesRunnerFilters(runner, config.Filters) {
			runners = append(runners, runner)
		}
	}
	if err := iter.Err(); err != nil {
		resp.Diagnostics.AddError("Failed to list runners", err.Error())
		return
	}

	state := mapRunnersToDataSourceModel(runners)
	state.Filters = config.Filters
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func matchesRunnerFilters(runner gitpod.Runner, filters []runnersFilterModel) bool {
	for _, f := range filters {
		var fieldValue string
		switch f.Name.ValueString() {
		case "name":
			fieldValue = runner.Name
		case "runner_manager_id":
			fieldValue = runner.RunnerManagerID
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

func mapRunnersToDataSourceModel(runners []gitpod.Runner) runnersDataSourceModel {
	sort.Slice(runners, func(i, j int) bool {
		return runners[i].RunnerID < runners[j].RunnerID
	})

	state := runnersDataSourceModel{
		Runners: make([]runnerListItemModel, 0, len(runners)),
	}

	for _, runner := range runners {
		state.Runners = append(state.Runners, runnerListItemModel{
			ID:              types.StringValue(runner.RunnerID),
			Name:            types.StringValue(runner.Name),
			ProviderType:    types.StringValue(string(runner.Provider)),
			RunnerManagerID: stringValueOrNull(runner.RunnerManagerID),
			Status:          runnerStatusObjectValue(runner.Status),
		})
	}

	return state
}

type runnersDataSourceModel struct {
	Filters []runnersFilterModel  `tfsdk:"filter"`
	Runners []runnerListItemModel `tfsdk:"runners"`
}

type runnersFilterModel struct {
	Name   types.String   `tfsdk:"name"`
	Values []types.String `tfsdk:"values"`
}

type runnerListItemModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	ProviderType    types.String `tfsdk:"provider_type"`
	RunnerManagerID types.String `tfsdk:"runner_manager_id"`
	Status          types.Object `tfsdk:"status"`
}
