package provider

import (
	"context"
	"testing"
	"time"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMapServiceAccountToModel(t *testing.T) {
	account := &v1.ServiceAccount{
		Id:             "sa-123",
		OrganizationId: "org-1",
		Name:           "ci",
		Description:    "CI pipeline",
		Creator: &v1.Subject{
			Id:        "user-1",
			Principal: v1.Principal_PRINCIPAL_USER,
		},
		CreatedAt:  timestamppb.New(time.Date(2026, time.March, 2, 15, 4, 5, 0, time.UTC)),
		ValidUntil: timestamppb.New(time.Date(2027, time.January, 31, 0, 0, 0, 0, time.UTC)),
	}

	got := mapServiceAccountToModel(account, serviceAccountModel{})

	assert.Equal(t, "sa-123", got.ID.ValueString())
	assert.Equal(t, "org-1", got.OrganizationID.ValueString())
	assert.Equal(t, "ci", got.Name.ValueString())
	assert.Equal(t, "CI pipeline", got.Description.ValueString())
	assert.Equal(t, "user-1", got.CreatorID.ValueString())
	assert.Equal(t, "PRINCIPAL_USER", got.CreatorPrincipal.ValueString())
	assert.Equal(t, "2026-03-02T15:04:05Z", got.CreatedAt.ValueString())
	assert.Equal(t, "2027-01-31T00:00:00Z", got.ValidUntil.ValueString())
}

func TestParseValidUntil(t *testing.T) {
	t.Run("missing expiry sends no timestamp rather than the zero time", func(t *testing.T) {
		var diags diag.Diagnostics
		got := parseValidUntil(types.StringNull(), &diags)
		require.False(t, diags.HasError())
		assert.Nil(t, got)
	})

	t.Run("RFC3339 expiry is converted", func(t *testing.T) {
		var diags diag.Diagnostics
		got := parseValidUntil(types.StringValue("2027-01-31T00:00:00Z"), &diags)
		require.False(t, diags.HasError())
		require.NotNil(t, got)
		assert.Equal(t, time.Date(2027, time.January, 31, 0, 0, 0, 0, time.UTC), got.AsTime().UTC())
	})

	t.Run("an unknown expiry sends no timestamp", func(t *testing.T) {
		var diags diag.Diagnostics
		got := parseValidUntil(types.StringUnknown(), &diags)
		require.False(t, diags.HasError())
		assert.Nil(t, got)
	})

	t.Run("a date without a time is rejected rather than sent as no expiry", func(t *testing.T) {
		var diags diag.Diagnostics
		got := parseValidUntil(types.StringValue("2027-01-31"), &diags)
		require.True(t, diags.HasError())
		assert.Nil(t, got)
	})
}

func TestServiceAccountSchema_ValidUntilIsRequiredAndForcesReplacement(t *testing.T) {
	var resp resource.SchemaResponse
	NewServiceAccountResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError())

	ctx := context.Background()
	validUntil, ok := resp.Schema.Attributes["valid_until"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, validUntil.IsRequired())
	require.Len(t, validUntil.PlanModifiers, 1)

	raw := nonNullRawValue(ctx, t, resp.Schema)
	modifyResp := planmodifier.StringResponse{}
	validUntil.PlanModifiers[0].PlanModifyString(ctx, planmodifier.StringRequest{
		State:       tfsdk.State{Raw: raw, Schema: resp.Schema},
		Plan:        tfsdk.Plan{Raw: raw, Schema: resp.Schema},
		StateValue:  types.StringValue("2027-01-31T00:00:00Z"),
		PlanValue:   types.StringValue("2028-01-31T00:00:00Z"),
		ConfigValue: types.StringValue("2028-01-31T00:00:00Z"),
	}, &modifyResp)

	assert.True(t, modifyResp.RequiresReplace)
}

func nonNullRawValue(ctx context.Context, t *testing.T, s schema.Schema) tftypes.Value {
	t.Helper()

	objectType, ok := s.Type().TerraformType(ctx).(tftypes.Object)
	require.True(t, ok)

	attributes := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		require.True(t, attributeType.Is(tftypes.String), name)
		attributes[name] = tftypes.NewValue(attributeType, "value")
	}

	return tftypes.NewValue(objectType, attributes)
}

func TestTimestampValueWithPrior(t *testing.T) {
	utc := timestamppb.New(time.Date(2027, time.January, 31, 0, 0, 0, 0, time.UTC))

	t.Run("keeps an equivalent configured offset", func(t *testing.T) {
		got := timestampValueWithPrior(utc, types.StringValue("2027-01-31T01:00:00+01:00"))
		assert.Equal(t, "2027-01-31T01:00:00+01:00", got.ValueString())
	})

	t.Run("uses the API value when the instant differs", func(t *testing.T) {
		got := timestampValueWithPrior(utc, types.StringValue("2027-02-01T00:00:00Z"))
		assert.Equal(t, "2027-01-31T00:00:00Z", got.ValueString())
	})

	t.Run("no expiry stays null", func(t *testing.T) {
		got := timestampValueWithPrior(nil, types.StringNull())
		assert.True(t, got.IsNull())
	})
}
