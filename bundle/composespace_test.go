package bundle

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

func spaceObservation(fields map[string]*types.Value) *model.SmartBlockSnapshotBase {
	details := map[string]*types.Value{
		"id":             strVal("bafyreiobservedspace"),
		"layout":         numVal(9),
		"resolvedLayout": numVal(10),
		"isHidden":       boolVal(true),
	}
	for key, value := range fields {
		details[key] = value
	}
	return &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{{
			Id: "bafyreiobservedspace",
			Content: &model.BlockContentOfSmartblock{
				Smartblock: &model.BlockContentSmartblock{},
			},
		}},
		Details: detFields(details),
	}
}

type spaceComposeResult struct {
	index      []byte
	dictionary []byte
	stats      Stats
	err        error
}

func composeSpaceObservations(t *testing.T, fallback string, concurrent bool, observations ...*model.SmartBlockSnapshotBase) spaceComposeResult {
	t.Helper()
	composer := NewComposer(anyblockjson.Options{}, fallback)
	if concurrent {
		type observationResult struct {
			omitted bool
			issues  []Issue
		}
		results := make(chan observationResult, len(observations))
		var wg sync.WaitGroup
		for _, observation := range observations {
			observation := observation
			wg.Add(1)
			go func() {
				defer wg.Done()
				omitted, issues := composer.Observe(model.SmartBlockType_Workspace, observation)
				results <- observationResult{omitted: omitted, issues: issues}
			}()
		}
		wg.Wait()
		close(results)
		for result := range results {
			require.True(t, result.omitted)
			require.Empty(t, result.issues)
		}
	} else {
		for _, observation := range observations {
			omitted, issues := composer.Observe(model.SmartBlockType_Workspace, observation)
			require.True(t, omitted)
			require.Empty(t, issues)
		}
	}
	index, dictionary, stats, err := composer.Finish()
	return spaceComposeResult{index: index, dictionary: dictionary, stats: stats, err: err}
}

func decodeComposedIndex(t *testing.T, result spaceComposeResult) *anyblockjson.Index {
	t.Helper()
	require.NoError(t, result.err)
	require.NotNil(t, result.index)
	require.NotNil(t, result.dictionary)
	idx, err := anyblockjson.UnmarshalIndex(result.index, anyblockjson.Options{})
	require.NoError(t, err)
	return idx
}

func TestIdenticalSpaceCandidatesDeduplicateForEveryLiftedField(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string]*types.Value
		check  func(*testing.T, *anyblockjson.Index)
	}{
		{
			name:   "name",
			fields: map[string]*types.Value{"name": strVal("Observed")},
			check:  func(t *testing.T, idx *anyblockjson.Index) { assert.Equal(t, "Observed", idx.Name) },
		},
		{
			name:   "description",
			fields: map[string]*types.Value{"description": strVal("About")},
			check:  func(t *testing.T, idx *anyblockjson.Index) { assert.Equal(t, "About", idx.Description) },
		},
		{
			name:   "homepage",
			fields: map[string]*types.Value{"homepage": strVal("widgets")},
			check: func(t *testing.T, idx *anyblockjson.Index) {
				assert.Equal(t, anyblockjson.HomepageWidgets, idx.Homepage)
			},
		},
		{
			name:   "icon",
			fields: map[string]*types.Value{"iconEmoji": strVal("🧭")},
			check: func(t *testing.T, idx *anyblockjson.Index) {
				require.NotNil(t, idx.Icon)
				assert.Equal(t, "emoji", idx.Icon.Format)
				assert.Equal(t, "🧭", idx.Icon.Emoji)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first := spaceObservation(tc.fields)
			second := spaceObservation(tc.fields)
			forward := composeSpaceObservations(t, "Fallback", false, first, second)
			reverse := composeSpaceObservations(t, "Fallback", false, second, first)
			require.NoError(t, forward.err)
			require.NoError(t, reverse.err)
			assert.True(t, bytes.Equal(forward.index, reverse.index))
			assert.True(t, bytes.Equal(forward.dictionary, reverse.dictionary))
			tc.check(t, decodeComposedIndex(t, forward))
		})
	}
}

func TestComplementarySpaceCandidatesMergeInEveryOrder(t *testing.T) {
	nameAndIcon := spaceObservation(map[string]*types.Value{
		"name":      strVal("Observed"),
		"iconEmoji": strVal("🧭"),
	})
	descriptionAndHomepage := spaceObservation(map[string]*types.Value{
		"description": strVal("About"),
		"homepage":    strVal("graph"),
	})

	forward := composeSpaceObservations(t, "Fallback", false, nameAndIcon, descriptionAndHomepage)
	reverse := composeSpaceObservations(t, "Fallback", false, descriptionAndHomepage, nameAndIcon)
	require.NoError(t, forward.err)
	require.NoError(t, reverse.err)
	assert.Equal(t, string(forward.index), string(reverse.index))
	assert.Equal(t, string(forward.dictionary), string(reverse.dictionary))

	idx := decodeComposedIndex(t, forward)
	assert.Equal(t, "Observed", idx.Name)
	assert.Equal(t, "About", idx.Description)
	assert.Equal(t, anyblockjson.HomepageGraph, idx.Homepage)
	require.NotNil(t, idx.Icon)
	assert.Equal(t, "🧭", idx.Icon.Emoji)
}

func TestEveryLiftedFieldConflictIsOrderIndependentAndAtomic(t *testing.T) {
	tests := []struct {
		name string
		one  map[string]*types.Value
		two  map[string]*types.Value
		want string
	}{
		{"name", map[string]*types.Value{"name": strVal("Zulu")}, map[string]*types.Value{"name": strVal("Alpha")}, `name=["Alpha", "Zulu"]`},
		{"description", map[string]*types.Value{"description": strVal("Zulu")}, map[string]*types.Value{"description": strVal("Alpha")}, `description=["Alpha", "Zulu"]`},
		{"homepage", map[string]*types.Value{"homepage": strVal("widgets")}, map[string]*types.Value{"homepage": strVal("graph")}, `homepage=["_graph", "_widgets"]`},
		{"icon", map[string]*types.Value{"iconEmoji": strVal("🧭")}, map[string]*types.Value{"iconEmoji": strVal("🔥")}, `icon=[{"format":"emoji","emoji":"🔥","file":"","name":"","color":null}, {"format":"emoji","emoji":"🧭","file":"","name":"","color":null}]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			one := spaceObservation(tc.one)
			two := spaceObservation(tc.two)
			forward := composeSpaceObservations(t, "Alpha", false, one, two)
			reverse := composeSpaceObservations(t, "Alpha", false, two, one)
			for _, result := range []spaceComposeResult{forward, reverse} {
				require.Error(t, result.err)
				assert.Contains(t, result.err.Error(), tc.want)
				assert.Nil(t, result.index)
				assert.Nil(t, result.dictionary)
				assert.Equal(t, 2, result.stats.OmittedDocs)
			}
			assert.Equal(t, forward.err.Error(), reverse.err.Error())
			for run := 0; run < 5; run++ {
				concurrent := composeSpaceObservations(t, "Alpha", true, one, two)
				require.EqualError(t, concurrent.err, forward.err.Error(), "concurrent run %d", run)
				assert.Nil(t, concurrent.index)
				assert.Nil(t, concurrent.dictionary)
			}
		})
	}
}

func TestMultiFieldConflictErrorAndNilArtifactsAreDeterministic(t *testing.T) {
	one := spaceObservation(map[string]*types.Value{
		"name": strVal("Zulu"), "description": strVal("Zulu"),
		"homepage": strVal("widgets"), "iconEmoji": strVal("🧭"),
	})
	two := spaceObservation(map[string]*types.Value{
		"name": strVal("Alpha"), "description": strVal("Alpha"),
		"homepage": strVal("graph"), "iconEmoji": strVal("🔥"),
	})
	want := `conflicting observed space settings: description=["Alpha", "Zulu"]; homepage=["_graph", "_widgets"]; icon=[{"format":"emoji","emoji":"🔥","file":"","name":"","color":null}, {"format":"emoji","emoji":"🧭","file":"","name":"","color":null}]; name=["Alpha", "Zulu"]`
	for i, observations := range [][]*model.SmartBlockSnapshotBase{{one, two}, {two, one}} {
		result := composeSpaceObservations(t, "Alpha", false, observations...)
		require.EqualError(t, result.err, want, "permutation %d", i)
		assert.Nil(t, result.index)
		assert.Nil(t, result.dictionary)
	}
}

func TestFallbackIsAppliedAfterObservedNameResolution(t *testing.T) {
	t.Run("unnamed observation uses fallback", func(t *testing.T) {
		result := composeSpaceObservations(t, "Fallback", false, spaceObservation(map[string]*types.Value{
			"description": strVal("Observed without a name"),
		}))
		idx := decodeComposedIndex(t, result)
		assert.Equal(t, "Fallback", idx.Name)
		assert.Equal(t, "Observed without a name", idx.Description)
	})

	t.Run("observed singleton beats fallback", func(t *testing.T) {
		result := composeSpaceObservations(t, "Fallback", false, spaceObservation(map[string]*types.Value{
			"name": strVal("Observed"),
		}))
		assert.Equal(t, "Observed", decodeComposedIndex(t, result).Name)
	})

	t.Run("fallback matching one candidate does not resolve a conflict", func(t *testing.T) {
		result := composeSpaceObservations(t, "Alpha", false,
			spaceObservation(map[string]*types.Value{"name": strVal("Alpha")}),
			spaceObservation(map[string]*types.Value{"name": strVal("Beta")}),
		)
		require.Error(t, result.err)
		assert.Nil(t, result.index)
		assert.Nil(t, result.dictionary)
	})

	t.Run("unused composer remains nil", func(t *testing.T) {
		result := composeSpaceObservations(t, "Fallback", false)
		require.NoError(t, result.err)
		assert.Nil(t, result.index)
		assert.Nil(t, result.dictionary)
	})
}

func TestConcurrentAggregationMatchesSequentialBytesAndErrors(t *testing.T) {
	complementary := []*model.SmartBlockSnapshotBase{
		spaceObservation(map[string]*types.Value{"name": strVal("Observed")}),
		spaceObservation(map[string]*types.Value{"description": strVal("About")}),
		spaceObservation(map[string]*types.Value{"homepage": strVal("widgets")}),
		spaceObservation(map[string]*types.Value{"iconEmoji": strVal("🧭")}),
	}
	var many []*model.SmartBlockSnapshotBase
	for i := 0; i < 32; i++ {
		many = append(many, complementary...)
	}
	sequential := composeSpaceObservations(t, "Fallback", false, many...)
	require.NoError(t, sequential.err)
	for i := 0; i < 10; i++ {
		concurrent := composeSpaceObservations(t, "Fallback", true, many...)
		require.NoError(t, concurrent.err)
		assert.Equal(t, string(sequential.index), string(concurrent.index), "run %d", i)
		assert.Equal(t, string(sequential.dictionary), string(concurrent.dictionary), "run %d", i)
	}

	conflicting := append(append([]*model.SmartBlockSnapshotBase{}, many...),
		spaceObservation(map[string]*types.Value{"name": strVal("Other")}))
	want := `conflicting observed space settings: name=["Observed", "Other"]`
	for i := 0; i < 10; i++ {
		result := composeSpaceObservations(t, "Fallback", true, conflicting...)
		require.EqualError(t, result.err, want, fmt.Sprintf("run %d", i))
		assert.Nil(t, result.index)
		assert.Nil(t, result.dictionary)
	}
}
