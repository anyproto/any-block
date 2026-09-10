package bundle

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

// An option's stored `apiObjectKey` travels on its vocabulary entry. It used
// to be classified as install machinery, on the reading that the app
// regenerates it from the name — and the app does have such a rule, but no
// restore runs it: an imported option is written straight into its tree and
// the create path that mints an api key is never reached, so the option
// lands with no api key at all and the public API addresses it by a
// hash-derived local key instead of the spelling callers wrote.
//
// Reproducibility was never the question. Measured over a 159-space corpus:
// 471 options that reach a dictionary carry a stored api key, and for 16 of
// them no derivation of the name reproduces it — `Canceled` stored as
// `cancelled`, `Product` as `produc`, `Awareness` as `discovery` — because
// an api key does not follow a rename. The other 455 would be regenerable
// if anything regenerated them.
//
// How this can fail: read the key but write it nowhere (the entry states a
// vocabulary the API cannot address); leave `apiObjectKey` classified as an
// install artifact (the entry states it AND the report says it was
// dropped); drop it from the canonical bare-string form's condition (an
// option carrying only an api key is written as a bare name and the key is
// lost at the writer instead of at the composer).
func TestComposerCarriesAnOptionsApiKey(t *testing.T) {
	// given
	c := newComposer(t, anyblockjson.Options{}, "Board")
	option := optionSnapshot("bafyopt", "status", "Canceled", "red", "63454af2")
	option.Details.Fields["apiObjectKey"] = strVal("cancelled")
	plain := optionSnapshot("bafyopt2", "status", "Done", "lime", "63454af3")

	// when
	for _, o := range []*model.SmartBlockSnapshotBase{option, plain} {
		omitted, issues := c.Observe(model.SmartBlockType_STRelationOption, o)
		require.True(t, omitted)
		require.Empty(t, issues, "the entry states the api key, so nothing is unaccounted for")
	}
	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafyp")})}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
		[]byte(`{"formatVersion":"2.0","properties":{"status":["Canceled"]}}`)))
	_, dictData, _, err := c.Finish()
	require.NoError(t, err)

	// then
	assert.Contains(t, string(dictData), `"api_key": "cancelled"`)
	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	require.Len(t, dict.Properties, 1)
	require.Len(t, dict.Properties[0].Options, 2)
	byName := map[string]anyblockjson.OptionDefinition{}
	for _, o := range dict.Properties[0].Options {
		byName[o.Name] = o
	}
	assert.Equal(t, "cancelled", byName["Canceled"].ApiKey,
		"a stored api key does not follow a rename, so nothing downstream can rebuild it")
	assert.Empty(t, byName["Done"].ApiKey, "an option the store holds none for states none")
}

func TestOptionDescriptionNoteDoesNotHideOtherOmissions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		content  bool
		category IssueCategory
	}{
		{"description only", false, IssueOptionDescriptionOmitted},
		{"description and page content", true, IssueOptionContentOmitted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newComposer(t, anyblockjson.Options{}, "Synthetic")
			option := optionSnapshot("option-test", "tag", "Test option", "red", "test-option")
			option.Details.Fields["description"] = strVal("An optional description")
			if tc.content {
				option.Blocks = append(option.Blocks, &model.Block{Id: "note", Content: &model.BlockContentOfText{Text: &model.BlockContentText{Text: "Keep this content"}}})
			}
			omitted, issues := c.Observe(model.SmartBlockType_STRelationOption, option)
			require.True(t, omitted)
			require.Len(t, issues, 1)
			assert.Equal(t, tc.category, issues[0].Category)
			require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, &model.SmartBlockSnapshotBase{}, []byte(`{"formatVersion":"2.0","properties":{"tag":["Test option"]}}`)))
			_, dictionary, stats, err := c.Finish()
			require.NoError(t, err)
			assert.Equal(t, 1, stats.OptionsLifted)
			assert.Contains(t, string(dictionary), "Test option")
		})
	}
}
