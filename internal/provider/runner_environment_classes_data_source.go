package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/gitpod-io/gitpod-sdk-go/shared"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &runnerEnvironmentClassesDataSource{}

type runnerEnvironmentClassesDataSource struct {
	client *gitpod.Client
}

func NewRunnerEnvironmentClassesDataSource() datasource.DataSource {
	return &runnerEnvironmentClassesDataSource{}
}

func (d *runnerEnvironmentClassesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner_environment_classes"
}

func (d *runnerEnvironmentClassesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List environment classes available on a Gitpod runner.",
		Attributes: map[string]schema.Attribute{
			"runner_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner ID.",
			},
			"environment_classes": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Environment classes exposed by the runner.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Environment class ID.",
						},
						"display_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Human-readable environment class name.",
						},
						"description": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Human-readable environment class description.",
						},
						"configuration": schema.MapAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "Configuration values keyed by configuration name.",
						},
						"enabled": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the environment class can be used to create new environments.",
						},
						"runner_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Runner ID this environment class belongs to.",
						},
					},
				},
			},
		},
	}
}

func (d *runnerEnvironmentClassesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *runnerEnvironmentClassesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config runnerEnvironmentClassesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	runnerID := config.RunnerID.ValueString()

	_, err := d.client.Runners.Get(ctx, gitpod.RunnerGetParams{
		RunnerID: gitpod.F(runnerID),
	})
	if err != nil {
		var apiErr *gitpod.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			resp.Diagnostics.AddError("Runner not found",
				fmt.Sprintf("No runner found with ID %s", runnerID))
			return
		}

		resp.Diagnostics.AddError("Failed to read runner", err.Error())
		return
	}

	iter := d.client.Runners.Configurations.EnvironmentClasses.ListAutoPaging(ctx, gitpod.RunnerConfigurationEnvironmentClassListParams{
		Filter: gitpod.F(gitpod.RunnerConfigurationEnvironmentClassListParamsFilter{
			RunnerIDs: gitpod.F([]string{runnerID}),
		}),
		Pagination: gitpod.F(gitpod.RunnerConfigurationEnvironmentClassListParamsPagination{
			PageSize: gitpod.F(int64(100)),
		}),
	})

	environmentClasses := make([]shared.EnvironmentClass, 0)
	for iter.Next() {
		environmentClasses = append(environmentClasses, iter.Current())
	}
	if err := iter.Err(); err != nil {
		resp.Diagnostics.AddError("Failed to list runner environment classes", err.Error())
		return
	}

	state, diags := mapRunnerEnvironmentClassesToDataSourceModel(runnerID, environmentClasses)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func mapRunnerEnvironmentClassesToDataSourceModel(runnerID string, environmentClasses []shared.EnvironmentClass) (runnerEnvironmentClassesDataSourceModel, diag.Diagnostics) {
	sort.Slice(environmentClasses, func(i, j int) bool {
		return environmentClasses[i].ID < environmentClasses[j].ID
	})

	state := runnerEnvironmentClassesDataSourceModel{
		RunnerID:           types.StringValue(runnerID),
		EnvironmentClasses: make([]runnerEnvironmentClassDataSourceModel, 0, len(environmentClasses)),
	}

	var diags diag.Diagnostics
	for _, environmentClass := range environmentClasses {
		configurationValues := make(map[string]attr.Value, len(environmentClass.Configuration))
		for _, field := range environmentClass.Configuration {
			configurationValues[field.Key] = types.StringValue(field.Value)
		}

		configuration, configDiags := types.MapValue(types.StringType, configurationValues)
		diags.Append(configDiags...)

		state.EnvironmentClasses = append(state.EnvironmentClasses, runnerEnvironmentClassDataSourceModel{
			ID:            types.StringValue(environmentClass.ID),
			DisplayName:   stringValueOrNull(environmentClass.DisplayName),
			Description:   stringValueOrNull(environmentClass.Description),
			Configuration: configuration,
			Enabled:       types.BoolValue(environmentClass.Enabled),
			RunnerID:      types.StringValue(environmentClass.RunnerID),
		})
	}

	return state, diags
}

type runnerEnvironmentClassesDataSourceModel struct {
	RunnerID           types.String                            `tfsdk:"runner_id"`
	EnvironmentClasses []runnerEnvironmentClassDataSourceModel `tfsdk:"environment_classes"`
}

type runnerEnvironmentClassDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	DisplayName   types.String `tfsdk:"display_name"`
	Description   types.String `tfsdk:"description"`
	Configuration types.Map    `tfsdk:"configuration"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	RunnerID      types.String `tfsdk:"runner_id"`
}
