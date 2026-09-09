package anyblockjson

import (
	"encoding/json"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// legacytargetkey_test.go pins what happens to a legacy bare TYPE key stored
// in relationFormatObjectTypes, and pins SPEC §2d to it.
//
// §2d promised such a key "passes through verbatim in both directions"; §11
// described the identical case as a normalization — it "comes back as this
// space's type object id". Both cannot be built from: a reader implementing
// §2d keeps a bare key where the importer stores an id, and a round-trip
// verifier following §2d reports the documented normalization as data loss.
//
// The runtime settles it and §11 was right. Neither direction is verbatim:
// export writes the key's DERIVED reference `type-<key>` (§9), and import
// resolves it to the space's type object id whenever the TypeResolver
// capability can answer. Only an unanswerable key stays a key.

// legacyTypeKeyResolver answers for `page` and nothing else, the way a wired
// space's store answers for a type it holds.
type legacyTypeKeyResolver struct{ answers bool }

func (r legacyTypeKeyResolver) TypeIdByKey(key string) (string, bool) {
	if r.answers && key == "page" {
		return "typeidpage", true
	}
	return "", false
}

func (r legacyTypeKeyResolver) TypeKeyById(id string) (string, bool) {
	if r.answers && id == "typeidpage" {
		return "page", true
	}
	return "", false
}

func (legacyTypeKeyResolver) PropertyById(string) (PropertyDefinition, bool) {
	return PropertyDefinition{}, false
}
func (legacyTypeKeyResolver) PropertyId(PropertyDefinition) (string, bool) { return "", false }

// TestLegacyBareTargetTypeKeyIsRespelledNotPassedVerbatim is the runtime half.
//
// How this can fail: drop the TypeIdByKey lookup from the object_types reader
// in relationformat.go — the §2d promise implemented literally — and the wired
// case stores the bare key instead of the space's id.
func TestLegacyBareTargetTypeKeyIsRespelledNotPassedVerbatim(t *testing.T) {
	// given: a property snapshot holding the LEGACY shape — a bare type key
	// where the store otherwise speaks type object ids
	snapshot := func() *model.SmartBlockSnapshotBase {
		return &model.SmartBlockSnapshotBase{Details: fields(map[string]*types.Value{
			"relationKey":               str("zzlink"),
			"name":                      str("Link"),
			"relationFormat":            {Kind: &types.Value_NumberValue{NumberValue: float64(model.RelationFormat_object)}},
			"relationFormatObjectTypes": strList("page"),
		})}
	}

	cases := map[string]struct {
		opts       Options
		wantStored string
	}{
		// the case §2d got wrong: a wired space RESPELLS the key as its id
		"a wired resolver that knows the key": {
			opts:       Options{ResolveProperties: legacyTypeKeyResolver{answers: true}},
			wantStored: "typeidpage",
		},
		// and the two cases §2d's promise does hold for
		"a wired resolver that does not know it": {
			opts:       Options{ResolveProperties: legacyTypeKeyResolver{answers: false}},
			wantStored: "page",
		},
		"no resolver at all": {
			opts:       Options{},
			wantStored: "page",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// when
			data, err := Marshal(model.SmartBlockType_STRelation, snapshot(), tc.opts)
			require.NoError(t, err)

			// then: the WIRE spelling is the derived reference in every case,
			// never the bare stored key — so export is not verbatim either
			var doc struct {
				PropertySettings struct {
					ObjectTypes []string `json:"object_types"`
				} `json:"property_settings"`
			}
			require.NoError(t, json.Unmarshal(data, &doc))
			assert.Equal(t, []string{"type-page"}, doc.PropertySettings.ObjectTypes,
				"canonical export writes the key's derived reference (§9)")

			// and the STORED value coming back is the space's id where the
			// capability can answer, the key where it cannot
			_, back, err := Unmarshal(data, tc.opts)
			require.NoError(t, err)
			got := back.Details.Fields["relationFormatObjectTypes"].GetListValue().GetValues()
			require.Len(t, got, 1)
			assert.Equal(t, tc.wantStored, got[0].GetStringValue())
		})
	}
}

// TestSpecStatesOneRuleForLegacyTargetTypeKeys is the prose half: §2d and §11
// describe one stored value crossing one boundary, so they may not promise
// two different normalized snapshots for it.
func TestSpecStatesOneRuleForLegacyTargetTypeKeys(t *testing.T) {
	spec := readFormatDocumentation(t)["SPEC.md"]

	assert.NotContains(t, spec, "passes through\n**verbatim in both directions**",
		"§2d promised a pass-through the boundary does not perform")
	assert.Contains(t, spec, "does not pass through verbatim: it takes **three steps**",
		"§2d must describe the three steps the value actually takes")
	assert.Contains(t, spec, "resolves that key to **this space's type object id** whenever the capability",
		"§2d must state the import step §11 already states")
	assert.Contains(t, spec, "the §11 normalization, a respelling and not a rebinding",
		"§2d must point at §11 rather than contradict it")
	// §11's half of the agreement; if it moves, §2d cross-references nothing.
	assert.Contains(t, spec, "stored where the store speaks object ids comes back as this space's type\nobject id",
		"§11 must still carry the normalization §2d now defers to")
}

// §11 carried the retired rule verbatim. Commit a97ce35 replaced §2d's "passes
// through **verbatim in both directions**" with the three steps the value
// actually takes, and pointed §2d at §11 — but §11's own paragraph still said
// "export passes the key through verbatim (it is no id the resolver serves)".
// Export does no such thing: `relationformat.go` wraps the target keys in
// `typeKeyRefs`, so `page` crosses as `type-page`, which the runtime half
// above asserts for all three wirings including none at all. A reader landing
// in §11 first — which is where §2d now sends them — got the retired rule
// back.
//
// The pair also stated one population twice and disagreed: §2d "21 production
// entries", §11 "27 corpus relations", different units and different numbers
// for what reads as the same thing. Neither is derivable from the corpus at
// out-57f4add, which carries no property documents at all — a bundle writes
// none (§15 #23) — so the figure is cited from the one sweep that measured it
// rather than restated.
func TestSpecStatesOneVerbatimRuleAndOnePopulation(t *testing.T) {
	spec := readFormatDocumentation(t)["SPEC.md"]

	assert.NotContains(t, spec, "export passes the key through verbatim",
		"§11 must not restore the rule §2d retired")
	assert.NotContains(t, spec, "it is no id the\nresolver serves",
		"the key IS a key the resolver serves; it is not an id, which is a different sentence")
	assert.Contains(t, spec, "Export does not pass it through: it writes the key's derived\nreference",
		"§11 must state the export step §2d states")

	assert.NotContains(t, spec, "27 corpus relations",
		"§11 may not restate a population §2d measures, in another unit, from no named source")
	assert.Contains(t, spec, "21 bare-key entries in `relationFormatObjectTypes`",
		"§11 must cite the population §2d measured, in the unit §2d measured it in")
	assert.NotContains(t, spec, "(21 production entries)",
		"§2d must say what the 21 are entries OF, since §11 now cites them")
}
