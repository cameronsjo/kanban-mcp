package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

// numericIDPattern hardens client-supplied IDs at the tool boundary. Planka IDs
// are numeric snowflakes; rejecting anything else stops path-traversal /
// cross-resource injection (e.g. an id like "1/../../users") before it is ever
// interpolated into an API path (ported from commit 93da5a3).
const numericIDPattern = `^\d+$`

var numericIDRe = regexp.MustCompile(numericIDPattern)

// Enum value sets shared across managers, ported from index.ts / operations.
var (
	labelColors = []string{
		"berry-red", "pumpkin-orange", "lagoon-blue", "pink-tulip", "light-mud",
		"orange-peel", "bright-moss", "antique-blue", "dark-granite", "lagune-blue",
		"sunny-grass", "morning-sky", "light-orange", "midnight-blue", "tank-green",
		"gun-metal", "wet-moss", "red-burgundy", "light-concrete", "apricot-red",
		"desert-sand", "navy-blue", "egg-yellow", "coral-green", "light-cocoa",
	}
	membershipRoles = []string{"editor", "viewer"}
	listTypes       = []string{"active", "closed"}
)

// inferSchema builds the JSON schema for the input type In and applies the given
// mutators. The mutators set constraints the SDK's `jsonschema:""` struct tag
// CANNOT express (pattern, enum). Without them the ^\d+$ ID hardening and the
// enum guards silently regress, because the struct tag is description-only.
// AddTool honors a pre-set Tool.InputSchema and still validates inputs against
// it, so these constraints are enforced pre-handler (Layer A).
func inferSchema[In any](mutators ...func(*jsonschema.Schema)) (*jsonschema.Schema, error) {
	s, err := jsonschema.For[In](nil)
	if err != nil {
		return nil, err
	}
	for _, m := range mutators {
		m(s)
	}
	return s, nil
}

// applyIDPattern sets the ^\d+$ pattern on each named top-level property.
func applyIDPattern(fields ...string) func(*jsonschema.Schema) {
	return func(s *jsonschema.Schema) {
		for _, f := range fields {
			if p := s.Properties[f]; p != nil {
				p.Pattern = numericIDPattern
			}
		}
	}
}

// applyEnum constrains a top-level property to a fixed set of string values.
func applyEnum(field string, values []string) func(*jsonschema.Schema) {
	return func(s *jsonschema.Schema) {
		p := s.Properties[field]
		if p == nil {
			return
		}
		enum := make([]any, len(values))
		for i, v := range values {
			enum[i] = v
		}
		p.Enum = enum
	}
}

// applyNestedIDPattern sets the ^\d+$ pattern on a field nested inside an array
// property's item schema (e.g. tasks[].cardId), generalizing applyIDPattern one
// level deeper.
func applyNestedIDPattern(arrayField, nestedField string) func(*jsonschema.Schema) {
	return func(s *jsonschema.Schema) {
		arr := s.Properties[arrayField]
		if arr == nil || arr.Items == nil {
			return
		}
		if p := arr.Items.Properties[nestedField]; p != nil {
			p.Pattern = numericIDPattern
		}
	}
}

// --- Layer B: handler-side, action-conditional validation ---------------------
// A flat action-discriminated schema cannot express "create needs name+listId",
// so per-action requiredness lives in the handlers. requireID re-checks ^\d+$
// (belt-and-suspenders behind the schema pattern); requireName trims and rejects
// empty.

// requireID returns the value of a required ID field, re-validating ^\d+$.
func requireID(field string, v *string) (string, error) {
	if v == nil || *v == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if !numericIDRe.MatchString(*v) {
		return "", fmt.Errorf("%s must be a numeric Planka ID", field)
	}
	return *v, nil
}

// requireName trims a required name field and rejects empty/whitespace-only.
func requireName(field string, v *string) (string, error) {
	if v == nil {
		return "", fmt.Errorf("%s is required", field)
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return "", fmt.Errorf("%s cannot be empty", field)
	}
	return trimmed, nil
}

// requireString returns a required non-empty string with no numeric/trim
// constraint (e.g. comment text, label color already enum-guarded).
func requireString(field string, v *string) (string, error) {
	if v == nil || *v == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	return *v, nil
}

// requireFloat returns the value of a required numeric field (e.g. position).
func requireFloat(field string, v *float64) (float64, error) {
	if v == nil {
		return 0, fmt.Errorf("%s is required", field)
	}
	return *v, nil
}

// deref reads *p, or def when p is nil — the nil-safe read for optional pointer
// fields in the flat action structs.
func deref[T any](p *T, def T) T {
	if p != nil {
		return *p
	}
	return def
}

// ptr returns a pointer to v, for building optional request-body fields.
func ptr[T any](v T) *T { return &v }
