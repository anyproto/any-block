package anyblockjson

// shadowedbundledspelling_test.go pins the legend entry a bundled spelling
// owes when a LIVE custom property carries the same display name (§3). The
// writer's own vocabulary resolves the spelling bundled-first, so the two
// existing questions — "does the bundled table bind it" and "does the
// writer's vocabulary invert it" — both said yes and no entry was written.
// A reader planning from the bundle's dictionary sees two live claimants
// and refuses the document. Measured: one space renamed the bundled Tag to
// "Regs" and minted a custom property named Tag; 542 object documents
// spelled `Tag` with no legend line, every one refused on read.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

const customDescriptionKey = "68cdaa41e9223c9dc7ce5f30"

// shadowingSpaceVocabulary is the writer's side of that space: the bundled
// spelling resolves bundled-first, and the claimant set names both.
type shadowingSpaceVocabulary struct {
	BundledKeyVocabulary
	// claimantsOnly is the store vocabulary's shape: the space's own
	// claimants without the bundled binding added — one entry, not two.
	claimantsOnly bool
}

func (shadowingSpaceVocabulary) PropertySlug(key string) string {
	if key == customDescriptionKey {
		return "Description"
	}
	return BundledKeyVocabulary{}.PropertySlug(key)
}
func (v shadowingSpaceVocabulary) PropertyKeyCandidates(spelling string) []string {
	if spelling == "Description" {
		if v.claimantsOnly {
			return []string{customDescriptionKey}
		}
		return []string{customDescriptionKey, "description"}
	}
	if key, ok := BundledPropertyKeyByName(spelling); ok {
		return []string{key}
	}
	return nil
}
func (shadowingSpaceVocabulary) TypeKeyCandidates(spelling string) []string {
	if key, ok := BundledTypeKeyByName(spelling); ok {
		return []string{key}
	}
	return nil
}
func (shadowingSpaceVocabulary) TypePropertyKeys(string) []string      { return nil }
func (shadowingSpaceVocabulary) PropertyTermFacts(string) KeyTermFacts { return KeyTermFacts{} }
func (shadowingSpaceVocabulary) TypeTermFacts(string) KeyTermFacts     { return KeyTermFacts{} }

func TestExport_ABundledSpellingShadowedByACustomNameOwesALegendEntry(t *testing.T) {
	snap := &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{{Id: "o1",
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}},
		Details: fields(map[string]*types.Value{
			"id":          str("o1"),
			"description": str("the bundled one"),
		}),
	}

	for name, vocab := range map[string]shadowingSpaceVocabulary{
		"claimants plus the bundled binding": {},
		"the space's claimants alone":        {claimantsOnly: true},
	} {
		t.Run(name, func(t *testing.T) {
			data, err := Marshal(model.SmartBlockType_Page, snap, Options{Keys: vocab})
			require.NoError(t, err)

			doc := decodeEnvelope(t, data)
			assert.Equal(t, map[string]string{"Description": "description"}, doc.PropertyKeys,
				"the spelling is contested in this space, so the document says which key it means")
			assert.Contains(t, doc.Properties, "Description")
		})
	}
}
