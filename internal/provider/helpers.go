package provider

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func stringValueOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func mergeStringWithPrior(current string, prior types.String) types.String {
	if current != "" {
		return types.StringValue(current)
	}
	if !prior.IsNull() && !prior.IsUnknown() {
		return prior
	}
	return types.StringNull()
}

func stringValueOrPriorExplicitEmpty(current string, prior types.String) types.String {
	if current != "" {
		return types.StringValue(current)
	}
	if !prior.IsNull() && !prior.IsUnknown() && prior.ValueString() == "" {
		return types.StringValue("")
	}
	return types.StringNull()
}

func stringListValue(values []string) types.List {
	elems := make([]attr.Value, len(values))
	for i, v := range values {
		elems[i] = types.StringValue(v)
	}
	return types.ListValueMust(types.StringType, elems)
}

func timeValueOrNull(value *timestamppb.Timestamp) types.String {
	if value == nil {
		return types.StringNull()
	}
	converted := value.AsTime()
	if converted.IsZero() {
		return types.StringNull()
	}
	return types.StringValue(converted.Format(time.RFC3339Nano))
}

func mapUpdateWindowValues(window *v1.UpdateWindow) (types.Int64, types.Int64, bool) {
	if window == nil {
		return types.Int64Null(), types.Int64Null(), false
	}

	startHour := types.Int64Null()
	if window.StartHour != nil {
		startHour = types.Int64Value(int64(window.GetStartHour()))
	}

	endHour := types.Int64Null()
	if window.EndHour != nil {
		endHour = types.Int64Value(int64(window.GetEndHour()))
	}

	return startHour, endHour, true
}

// durationString formats a protobuf Duration the way the API renders it in
// JSON — seconds with a trailing "s", e.g. "3600s" — so round-tripping a
// configured timeout does not produce a spurious diff.
func durationString(value *durationpb.Duration) string {
	if value == nil {
		return ""
	}

	seconds := strconv.FormatFloat(value.AsDuration().Seconds(), 'f', 9, 64)
	seconds = strings.TrimSuffix(strings.TrimRight(seconds, "0"), ".")
	return seconds + "s"
}

// enumString renders a protobuf enum the way the REST SDK rendered it: the
// proto enum name, with the unspecified zero value mapping to the empty string
// so callers can keep treating it as absent.
func enumString[E interface {
	~int32
	String() string
}](value E) string {
	if value == 0 {
		return ""
	}
	return value.String()
}

// enumValue is the inverse of enumString: it resolves a proto enum name from
// configuration back to its numeric value. An unknown name is reported rather
// than silently sent as the unspecified zero value, which the API would accept
// while state kept claiming the configured name.
func enumValue[E ~int32](attribute, name string, values map[string]int32, diagnostics *diag.Diagnostics) E {
	// The unspecified zero value is not a configurable choice: protobuf omits it
	// on the wire and enumString reads it back as the empty string.
	value, ok := values[name]
	if !ok || value == 0 {
		diagnostics.AddError(
			fmt.Sprintf("Invalid %s", attribute),
			fmt.Sprintf("%q is not a recognised value for %s. Valid values: %s.",
				name, attribute, strings.Join(sortedEnumNames(values), ", ")),
		)
		return E(0)
	}
	return E(value)
}

// validatedHour checks a UTC hour attribute before it is narrowed to the
// protobuf field's 32-bit type, where an out-of-range value would silently wrap
// into a different but valid-looking hour.
func validatedHour(attribute string, value types.Int64, diagnostics *diag.Diagnostics) (int64, bool) {
	hour := value.ValueInt64()
	if hour < 0 || hour > 23 {
		diagnostics.AddError(
			fmt.Sprintf("Invalid %s", attribute),
			fmt.Sprintf("%s must be a UTC hour between 0 and 23, got %d.", attribute, hour),
		)
		return 0, false
	}
	return hour, true
}

// sortedEnumNames lists the accepted names for an enum, dropping the
// unspecified zero value since it is never a meaningful configuration input.
func sortedEnumNames(values map[string]int32) []string {
	names := make([]string, 0, len(values))
	for name, value := range values {
		if value == 0 {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// pairID joins the two identifiers of a resource that the API keys by a pair
// rather than by an ID of its own.
func pairID(first, second string) string {
	return first + "/" + second
}

// parsePairImportID splits an import ID of the form "<first>/<second>".
// format is the spelling shown to the user when the ID does not parse.
func parsePairImportID(importID, format string) (string, string, error) {
	first, second, ok := strings.Cut(importID, "/")
	if !ok || first == "" || second == "" {
		return "", "", fmt.Errorf("expected import identifier in the format %s", format)
	}

	return first, second, nil
}

// authenticatedOrganizationID resolves the organization the API key belongs to.
// Resources that are organization-scoped need the ID on create, and the API key
// is only ever valid for one organization.
func authenticatedOrganizationID(ctx context.Context, client *sdk.Client) (string, error) {
	if client == nil {
		return "", fmt.Errorf("provider is not configured")
	}

	result, err := client.Services.Identity.GetAuthenticatedIdentity(ctx, connect.NewRequest(&v1.GetAuthenticatedIdentityRequest{}))
	if err != nil {
		return "", fmt.Errorf("get authenticated identity: %w", err)
	}
	organizationID := result.Msg.GetOrganizationId()
	if organizationID == "" {
		return "", fmt.Errorf("authenticated identity has no organization ID")
	}
	return organizationID, nil
}

// isAPINotFound reports whether err is an API error signalling a missing
// resource, the connect equivalent of the HTTP 404 the REST SDK returned.
func isAPINotFound(err error) bool {
	return connect.CodeOf(err) == connect.CodeNotFound
}

// collectPaged drains a paginated list RPC. fetch is called with the page token
// ("" for the first page) and returns that page's items along with the token of
// the next page, which is empty once the last page has been read.
func collectPaged[T any](fetch func(token string) ([]T, string, error)) ([]T, error) {
	var all []T
	token := ""
	for {
		items, next, err := fetch(token)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if next == "" {
			return all, nil
		}
		token = next
	}
}

// clientFromProviderData extracts the *sdk.Client from the ProviderData
// passed to a resource or data source Configure call. Returns false (without
// adding a diagnostic) when ProviderData is nil — the framework calls
// Configure before provider configuration has completed. Returns false with a
// diagnostic when ProviderData is of an unexpected type.
func clientFromProviderData(providerData any, diagnostics *diag.Diagnostics) (*sdk.Client, bool) {
	if providerData == nil {
		return nil, false
	}
	client, ok := providerData.(*sdk.Client)
	if !ok {
		diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("Expected *sdk.Client, got %T", providerData))
		return nil, false
	}
	return client, true
}

var runnerStatusAttrTypes = map[string]attr.Type{
	"phase":   types.StringType,
	"message": types.StringType,
	"version": types.StringType,
	"region":  types.StringType,
}

// runnerStatusObjectValue converts a v1.RunnerStatus to a types.Object
// using the shared attribute-type map.
func runnerStatusObjectValue(status *v1.RunnerStatus) types.Object {
	obj, _ := types.ObjectValue(runnerStatusAttrTypes, map[string]attr.Value{
		"phase":   types.StringValue(enumString(status.GetPhase())),
		"message": types.StringValue(status.GetMessage()),
		"version": types.StringValue(status.GetVersion()),
		"region":  types.StringValue(status.GetRegion()),
	})
	return obj
}
