// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"reflect"
	"strings"
	"testing"
)

func TestRenderAliasUsesCanonicalSourceOnce(t *testing.T) {
	t.Parallel()

	source := image.NewRGBA(image.Rect(-3, 5, 1, 9))
	for y := 6; y < 8; y++ {
		for x := -2; x < 0; x++ {
			source.SetRGBA(x, y, color.RGBA{R: 210, G: 30, A: 255})
		}
	}
	layout := &Layout{
		Width: 4, Height: 4,
		Render: &RenderSettings{TrimTransparent: true},
		Placements: []Placement{
			{
				ID:             "canonical",
				X:              1,
				Y:              1,
				Width:          2,
				Height:         2,
				OriginalWidth:  4,
				OriginalHeight: 4,
				OffsetX:        1,
				OffsetY:        1,
			},
			{
				ID:             "alias",
				AliasOf:        "canonical",
				X:              1,
				Y:              1,
				Width:          2,
				Height:         2,
				OriginalWidth:  6,
				OriginalHeight: 4,
				OffsetX:        3,
				OffsetY:        1,
			},
		},
	}

	calls := make([]string, 0, 1)
	got, err := RenderWithLoader(layout, func(id string) (image.Image, error) {
		calls = append(calls, id)
		if id != "canonical" {
			return nil, ErrSourceNotFound
		}

		return source, nil
	})

	if err != nil {
		t.Fatalf("RenderWithLoader: %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"canonical"}) {
		t.Fatalf("loader calls = %v, want canonical once", calls)
	}

	eager, err := Render(layout, []Source{{ID: "canonical", Image: source}})
	if err != nil {
		t.Fatalf("Render with canonical source only: %v", err)
	}
	for y := 1; y < 3; y++ {
		for x := 1; x < 3; x++ {
			if pixel := got.(*image.RGBA).RGBAAt(x, y); pixel != (color.RGBA{R: 210, G: 30, A: 255}) {
				t.Errorf("pixel at (%d,%d) = %+v", x, y, pixel)
			}
			if pixel := eager.(*image.RGBA).RGBAAt(x, y); pixel != (color.RGBA{R: 210, G: 30, A: 255}) {
				t.Errorf("eager pixel at (%d,%d) = %+v", x, y, pixel)
			}
		}
	}
}

func TestRenderValidatesAliasesBeforeAllocatingAtlas(t *testing.T) {
	t.Parallel()

	validCanonical := Placement{ID: "canonical", X: 0, Y: 0, Width: 1, Height: 1}
	validAlias := Placement{ID: "alias", AliasOf: "canonical", X: 0, Y: 0, Width: 1, Height: 1}
	tests := []struct {
		name      string
		layout    Layout
		wantError error
	}{
		{
			name: "missing canonical",
			layout: Layout{Width: 8192, Height: 8192, Placements: []Placement{
				validCanonical,
				{ID: "alias", AliasOf: "missing", X: 0, Y: 0, Width: 1, Height: 1},
			}},
			wantError: ErrInvalidLayout,
		},
		{
			name: "self alias",
			layout: Layout{Width: 2, Height: 2, Placements: []Placement{
				{ID: "self", AliasOf: "self", X: 0, Y: 0, Width: 1, Height: 1},
			}},
			wantError: ErrInvalidLayout,
		},
		{
			name: "alias to alias",
			layout: Layout{Width: 2, Height: 2, Placements: []Placement{
				validCanonical,
				validAlias,
				{ID: "second-alias", AliasOf: "alias", X: 0, Y: 0, Width: 1, Height: 1},
			}},
			wantError: ErrInvalidLayout,
		},
		{
			name: "duplicate IDs",
			layout: Layout{Width: 2, Height: 2, Placements: []Placement{
				validCanonical,
				validCanonical,
			}},
			wantError: ErrInvalidLayout,
		},
		{
			name: "alias position differs",
			layout: Layout{Width: 2, Height: 2, Placements: []Placement{
				validCanonical,
				{ID: "alias", AliasOf: "canonical", X: 1, Y: 0, Width: 1, Height: 1},
			}},
			wantError: ErrInvalidLayout,
		},
		{
			name: "alias width differs",
			layout: Layout{Width: 2, Height: 2, Placements: []Placement{
				validCanonical,
				{ID: "alias", AliasOf: "canonical", X: 0, Y: 0, Width: 2, Height: 1},
			}},
			wantError: ErrInvalidLayout,
		},
		{
			name: "alias height differs",
			layout: Layout{Width: 2, Height: 2, Placements: []Placement{
				validCanonical,
				{ID: "alias", AliasOf: "canonical", X: 0, Y: 0, Width: 1, Height: 2},
			}},
			wantError: ErrInvalidLayout,
		},
		{
			name: "alias rotation differs",
			layout: Layout{Width: 2, Height: 2, Placements: []Placement{
				{ID: "canonical", X: 0, Y: 0, Width: 1, Height: 2},
				{ID: "alias", AliasOf: "canonical", X: 0, Y: 0, Width: 1, Height: 2, Rotated: true},
			}},
			wantError: ErrInvalidLayout,
		},
		{
			name: "alias crop metadata out of bounds",
			layout: Layout{Width: 2, Height: 2, Placements: []Placement{
				validCanonical,
				{
					ID:             "alias",
					AliasOf:        "canonical",
					X:              0,
					Y:              0,
					Width:          1,
					Height:         1,
					OriginalWidth:  1,
					OriginalHeight: 1,
					OffsetX:        1,
				},
			}},
			wantError: ErrInvalidLayout,
		},
		{
			name: "canonical overlap",
			layout: Layout{Width: 2, Height: 2, Placements: []Placement{
				validCanonical,
				{ID: "other", X: 0, Y: 0, Width: 1, Height: 1},
			}},
			wantError: ErrInvalidLayout,
		},
		{
			name: "reserved padding overlap",
			layout: Layout{
				Width: 8, Height: 4,
				Render: &RenderSettings{Padding: 1},
				Placements: []Placement{
					{ID: "left", X: 1, Y: 1, Width: 2, Height: 2},
					{ID: "right", X: 4, Y: 1, Width: 2, Height: 2},
				},
			},
			wantError: ErrInvalidLayout,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			loaderCalled := false
			_, err := RenderWithLoader(&test.layout, func(string) (image.Image, error) {
				loaderCalled = true
				return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
			})
			if !errors.Is(err, test.wantError) {
				t.Fatalf("RenderWithLoader error = %v, want %v", err, test.wantError)
			}
			if loaderCalled {
				t.Fatal("loader called before alias preflight completed")
			}
		})
	}
}

func TestPlacementAliasJSONAndYAMLTags(t *testing.T) {
	t.Parallel()

	placement := Placement{ID: "alias", AliasOf: "canonical", X: 1, Y: 2, Width: 3, Height: 4}
	encoded, err := json.Marshal(placement)
	if err != nil {
		t.Fatalf("Marshal alias: %v", err)
	}
	if !strings.Contains(string(encoded), `"alias_of":"canonical"`) {
		t.Fatalf("alias JSON = %s", encoded)
	}

	var decoded Placement
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal alias: %v", err)
	}
	if decoded != placement {
		t.Fatalf("JSON alias round trip = %+v, want %+v", decoded, placement)
	}

	legacy := `{"id":"sprite","x":0,"y":0,"width":1,"height":1,"rotated":false}`
	decoded = Placement{}
	if err := json.Unmarshal([]byte(legacy), &decoded); err != nil {
		t.Fatalf("Unmarshal legacy placement: %v", err)
	}
	if decoded.AliasOf != "" {
		t.Fatalf("legacy AliasOf = %q, want empty", decoded.AliasOf)
	}
	field, ok := reflect.TypeFor[Placement]().FieldByName("AliasOf")
	if !ok || field.Tag.Get("yaml") != "alias_of,omitempty" {
		t.Fatalf("AliasOf YAML tag = %q, want alias_of,omitempty", field.Tag.Get("yaml"))
	}

	optionField, ok := reflect.TypeFor[Options]().FieldByName("Deduplicate")
	if !ok || optionField.Tag.Get("yaml") != "deduplicate,omitempty" {
		t.Fatalf("Deduplicate YAML tag = %q, want deduplicate,omitempty", optionField.Tag.Get("yaml"))
	}

	plainJSON, err := json.Marshal(Placement{ID: "plain", Width: 1, Height: 1})
	if err != nil {
		t.Fatalf("Marshal canonical: %v", err)
	}
	if strings.Contains(string(plainJSON), "alias_of") {
		t.Fatalf("canonical JSON unexpectedly contains alias_of: %s", plainJSON)
	}

	defaultOptionsJSON, err := json.Marshal(Options{})
	if err != nil {
		t.Fatalf("Marshal default options: %v", err)
	}
	if strings.Contains(string(defaultOptionsJSON), "deduplicate") {
		t.Fatalf("default options unexpectedly contain deduplicate: %s", defaultOptionsJSON)
	}
}
