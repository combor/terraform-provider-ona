package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &runnerTokenDataSource{}

type runnerTokenDataSource struct {
	client *sdk.Client
}

func NewRunnerTokenDataSource() datasource.DataSource {
	return &runnerTokenDataSource{}
}

func (d *runnerTokenDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner_token"
}

func (d *runnerTokenDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Retrieve a new authentication token for a Gitpod runner.

!> The ` + "`" + `exchange_token` + "`" + ` attribute is persisted to state.`,
		Attributes: map[string]schema.Attribute{
			"runner_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner ID.",
			},
			"exchange_token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "A one-time use token that should be exchanged by the runner for an access token, using the IdentityService.ExchangeToken rpc. The token expires after 24 hours.",
			},
		},
	}
}

func (d *runnerTokenDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	d.client = client
}

func (d *runnerTokenDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config runnerTokenDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	runnerID := config.RunnerID.ValueString()

	token, err := d.client.Services.Runner.CreateRunnerToken(ctx, connect.NewRequest(&v1.CreateRunnerTokenRequest{
		RunnerId: runnerID,
	}))
	if err != nil {
		if isAPINotFound(err) {
			resp.Diagnostics.AddError("Runner not found",
				fmt.Sprintf("No runner found with ID %s", runnerID))
			return
		}

		resp.Diagnostics.AddError("Failed to retrieve runner token", err.Error())
		return
	}

	state := mapRunnerTokenToDataSourceModel(runnerID, token.Msg.GetExchangeToken())
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func mapRunnerTokenToDataSourceModel(runnerID string, token string) runnerTokenDataSourceModel {
	return runnerTokenDataSourceModel{
		RunnerID:      types.StringValue(runnerID),
		ExchangeToken: types.StringValue(token),
	}
}

type runnerTokenDataSourceModel struct {
	RunnerID      types.String `tfsdk:"runner_id"`
	ExchangeToken types.String `tfsdk:"exchange_token"`
}
