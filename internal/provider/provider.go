package provider

import (
	"context"
	"os"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/gitpod-io/gitpod-sdk-go/option"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/combor/terraform-provider-ona/internal/datasource_runner"
	"github.com/combor/terraform-provider-ona/internal/resource_runner"
)

var _ provider.Provider = (*onaProvider)(nil)

type onaProvider struct{}

type onaProviderModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

func New() provider.Provider {
	return &onaProvider{}
}

func (p *onaProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ona"
}

func (p *onaProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage runners on ona.com.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "API key for ona.com. Can also be set via GITPOD_API_KEY environment variable.",
			},
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL for the API. Can also be set via GITPOD_BASE_URL environment variable.",
			},
		},
	}
}

func (p *onaProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config onaProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("GITPOD_API_KEY")
	if !config.APIKey.IsNull() {
		apiKey = config.APIKey.ValueString()
	}

	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing API Key",
			"Set the api_key provider attribute or the GITPOD_API_KEY environment variable.",
		)
		return
	}

	opts := []option.RequestOption{
		option.WithBearerToken(apiKey),
	}

	baseURL := os.Getenv("GITPOD_BASE_URL")
	if !config.BaseURL.IsNull() {
		baseURL = config.BaseURL.ValueString()
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := gitpod.NewClient(opts...)

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *onaProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resource_runner.NewRunnerResource,
	}
}

func (p *onaProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasource_runner.NewRunnerDataSource,
	}
}
