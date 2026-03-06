package provider

import (
	"context"
	"os"
	"testing"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"ona": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	if os.Getenv("GITPOD_API_KEY") == "" {
		t.Skip("GITPOD_API_KEY not set, skipping acceptance test")
	}
}

func TestProviderSchema_ExposesRetryAndTimeoutAttributes(t *testing.T) {
	p := &onaProvider{}
	var resp frameworkprovider.SchemaResponse

	p.Schema(context.Background(), frameworkprovider.SchemaRequest{}, &resp)

	maxRetries, ok := resp.Schema.Attributes["max_retries"]
	require.True(t, ok)
	assert.True(t, maxRetries.IsOptional())

	requestTimeout, ok := resp.Schema.Attributes["request_timeout"]
	require.True(t, ok)
	assert.True(t, requestTimeout.IsOptional())
}
