package anyblockjson

import (
	"fmt"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

type mapPropertyResolver map[string]PropertyDefinition

func (r mapPropertyResolver) PropertyById(id string) (PropertyDefinition, bool) {
	def, ok := r[id]
	return def, ok
}

func (mapPropertyResolver) PropertyId(PropertyDefinition) (string, bool) {
	return "", false
}

func TestTypeRendererErrorsUseEmittedDestinationIndex(t *testing.T) {
	valid := PropertyDefinition{Key: domain.RelationKey("valid"), Format: model.RelationFormat_longtext}
	invalid := PropertyDefinition{
		Key:     domain.RelationKey("invalid"),
		Format:  model.RelationFormat_status,
		Options: []OptionDefinition{{Name: "Broken", Color: "chartreuse"}},
	}
	// An empty stored key is deliberately dropped before rendering, so it
	// consumes no destination index.
	dropped := PropertyDefinition{Format: model.RelationFormat_longtext}

	tests := []struct {
		name      string
		ids       []string
		wantIndex int
	}{
		{name: "invalid first", ids: []string{"invalid"}, wantIndex: 0},
		{name: "valid then invalid", ids: []string{"valid", "invalid"}, wantIndex: 1},
		{name: "dropped then invalid", ids: []string{"dropped", "invalid"}, wantIndex: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := &model.SmartBlockSnapshotBase{
				Details: fields(map[string]*types.Value{
					"id":                   str("typeobj"),
					"recommendedRelations": strList(tc.ids...),
				}),
				ObjectTypes: []string{"ot-objectType"},
			}

			data, err := Marshal(model.SmartBlockType_STType, snapshot, Options{
				ResolveProperties: mapPropertyResolver{
					"valid":   valid,
					"invalid": invalid,
					"dropped": dropped,
				},
			})

			require.Error(t, err)
			assert.Nil(t, data, "a renderer refusal must never return partial bytes")
			assert.Contains(t, err.Error(), fmt.Sprintf("/type_settings/property_definitions/%d", tc.wantIndex))
			assert.Contains(t, err.Error(), "options[0].color")
		})
	}
}

func TestDictionaryAndTypeRendererPathConventionsAgree(t *testing.T) {
	invalid := PropertyDefinition{
		Key:     domain.RelationKey("invalid"),
		Format:  model.RelationFormat_status,
		Options: []OptionDefinition{{Name: "Broken", Color: "chartreuse"}},
	}

	data, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{invalid}}, Options{})
	require.Error(t, err)
	assert.Nil(t, data)
	assert.Contains(t, err.Error(), "/properties/0")
	assert.Contains(t, err.Error(), "options[0].color")
}
