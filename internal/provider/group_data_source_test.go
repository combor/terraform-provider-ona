package provider

import (
	"testing"
	"time"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMapGroupToDataSourceModel_MapsAllFields(t *testing.T) {
	got := mapGroupToDataSourceModel(&v1.Group{
		Id:             "group-123",
		Name:           "Engineering",
		Description:    "Engineering team",
		OrganizationId: "org-456",
		MemberCount:    5,
		DirectShare:    true,
		SystemManaged:  false,
		CreatedAt:      timestamppb.New(time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)),
		UpdatedAt:      timestamppb.New(time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC)),
	})

	assert.Equal(t, "group-123", got.ID.ValueString())
	assert.Equal(t, "Engineering", got.Name.ValueString())
	assert.Equal(t, "Engineering team", got.Description.ValueString())
	assert.Equal(t, "org-456", got.OrganizationID.ValueString())
	assert.Equal(t, int64(5), got.MemberCount.ValueInt64())
	assert.True(t, got.DirectShare.ValueBool())
	assert.False(t, got.SystemManaged.ValueBool())
	assert.Equal(t, "2026-01-02T03:04:05Z", got.CreatedAt.ValueString())
	assert.Equal(t, "2026-02-03T04:05:06Z", got.UpdatedAt.ValueString())
}

func TestMapGroupToDataSourceModel_EmptyOptionalFields(t *testing.T) {
	got := mapGroupToDataSourceModel(&v1.Group{
		Id: "group-789",
	})

	assert.Equal(t, "group-789", got.ID.ValueString())
	assert.True(t, got.Name.IsNull())
	assert.True(t, got.Description.IsNull())
	assert.True(t, got.OrganizationID.IsNull())
	assert.True(t, got.CreatedAt.IsNull())
	assert.True(t, got.UpdatedAt.IsNull())
}
