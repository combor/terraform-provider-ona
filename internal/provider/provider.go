package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/gitpod-io/gitpod-sdk-go/option"
)

var _ provider.Provider = &onaProvider{}

type onaProvider struct {
	version string
}

type onaProviderModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

func (p *onaProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ona"
	resp.Version = p.version
}

func (p *onaProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "API key. Falls back to `GITPOD_API_KEY` env var.",
			},
			"base_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "API base URL. Falls back to `GITPOD_BASE_URL` env var. Defaults to `https://app.gitpod.io/api`.",
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

	apiKey := config.APIKey.ValueString()
	if apiKey == "" {
		apiKey = os.Getenv("GITPOD_API_KEY")
	}
	if apiKey == "" {
		resp.Diagnostics.AddError("Missing API Key", "Set api_key in provider config or GITPOD_API_KEY env var.")
		return
	}

	baseURL := config.BaseURL.ValueString()
	if baseURL == "" {
		baseURL = os.Getenv("GITPOD_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://app.gitpod.io/api"
	}

	client := gitpod.NewClient(
		option.WithBearerToken(apiKey),
		option.WithBaseURL(baseURL),
	)

	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *onaProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewRunnerResource,
	}
}

func (p *onaProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectDataSource,
		NewRunnerDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &onaProvider{version: version}
	}
}
