package provider

import (
	"context"
	"math"
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

func TestInt64ToIntChecked(t *testing.T) {
	t.Run("accepts max value", func(t *testing.T) {
		got, err := int64ToIntChecked(7, 7)
		require.NoError(t, err)
		assert.Equal(t, 7, got)
	})

	t.Run("rejects value above max", func(t *testing.T) {
		_, err := int64ToIntChecked(8, 7)
		require.Error(t, err)
		assert.EqualError(t, err, "too large for this runtime (max 7)")
	})
}

func TestMaxRuntimeInt64(t *testing.T) {
	assert.Greater(t, maxRuntimeInt64(), int64(0))
	assert.LessOrEqual(t, maxRuntimeInt64(), int64(math.MaxInt64))
}
