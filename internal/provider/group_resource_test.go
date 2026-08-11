package provider

import (
	"context"
	"testing"
	"time"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMapGroupToResourceModel(t *testing.T) {
	group := &v1.Group{
		Id:             "group-123",
		OrganizationId: "org-1",
		Name:           "platform",
		Description:    "Platform engineers",
		MemberCount:    3,
		CreatedAt:      timestamppb.New(time.Date(2026, time.March, 2, 15, 4, 5, 0, time.UTC)),
		UpdatedAt:      timestamppb.New(time.Date(2026, time.March, 3, 15, 4, 5, 0, time.UTC)),
	}

	got := mapGroupToResourceModel(group, groupModel{})

	assert.Equal(t, "group-123", got.ID.ValueString())
	assert.Equal(t, "org-1", got.OrganizationID.ValueString())
	assert.Equal(t, "platform", got.Name.ValueString())
	assert.Equal(t, "Platform engineers", got.Description.ValueString())
	assert.Equal(t, int64(3), got.MemberCount.ValueInt64())
	assert.Equal(t, "2026-03-02T15:04:05Z", got.CreatedAt.ValueString())
	assert.Equal(t, "2026-03-03T15:04:05Z", got.UpdatedAt.ValueString())
}

func TestMapGroupToResourceModel_DescriptionSemantics(t *testing.T) {
	t.Run("unset description stays null", func(t *testing.T) {
		got := mapGroupToResourceModel(&v1.Group{Id: "group-123"}, groupModel{Description: types.StringNull()})
		assert.True(t, got.Description.IsNull())
	})

	t.Run("explicitly empty description is preserved", func(t *testing.T) {
		got := mapGroupToResourceModel(&v1.Group{Id: "group-123"}, groupModel{Description: types.StringValue("")})
		assert.False(t, got.Description.IsNull())
		assert.Equal(t, "", got.Description.ValueString())
	})
}

func TestGroupResourceSchema_DescriptionIsOptionalOnly(t *testing.T) {
	var resp resource.SchemaResponse
	NewGroupResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError())

	description := resp.Schema.Attributes["description"]
	// Optional-and-computed would let the API's empty string win over an
	// explicitly configured "", so description is optional only.
	assert.True(t, description.IsOptional())
	assert.False(t, description.IsComputed())
}
