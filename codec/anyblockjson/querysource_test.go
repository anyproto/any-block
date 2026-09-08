package anyblockjson

// querysource_test.go — the §6.2 `query_source` group: a query states what it
// ranges over on the ROOT, in two typed lists, and nowhere else.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// querySnapshot is a set document whose stored `setOf` holds the entries
// given, in the order given.
func querySnapshot(sources ...string) *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{
			{Id: "bafyreiquery", Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
		},
		Details: fields(map[string]*types.Value{
			"id":    str("bafyreiquery"),
			"name":  str("All Objects"),
			"setOf": strList(sources...),
		}),
	}
}

// A type target and a property target are two different things, and the
// document says which is which by the list it puts them in — the fact the
// flat `Set of` list could not state, because a property object id and a
// type object id are one grammar there (§6.2).
//
// How this can fail: write the group from one list; put a property target in
// `types`; leave the flat spelling in `properties` beside it.
func TestQuerySource_TypeAndPropertyTargetsSplitIntoTwoLists(t *testing.T) {
	opts := typeRefOptions()

	data, err := Marshal(model.SmartBlockType_Page, querySnapshot("typeid-page", "relid-dueDate"), opts)
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (§11 I1)")

	assert.Contains(t, string(data), `"query_source": {
    "types": [
      "type-page"
    ],
    "properties": [
      "dueDate"
    ]
  }`, "the group states each target in the list that says what it is")
	assert.NotContains(t, string(data), `"Set of"`,
		"the query source is lifted OUT of properties — one fact, one place (§6.2)")
}

// THE ORDERING RESIDUE, pinned. `setOf` is ONE ordered list and the group is
// two, so a stored value that interleaves types and properties cannot come
// back interleaved. It comes back PARTITIONED — every type in order, then
// every property in order — and that is a §11 normalization rather than a
// loss, for a reason §6.2 now states: several targets combine with OR, so the
// value is a union and the order of its operands means nothing.
//
// What must hold is that the movement converges in ONE generation, like the
// missing-reference rewrite: import stores the partitioned list, and every
// export after the first is byte-identical. Guarantees 2 and 3 are untouched.
//
// How this can fail: rebuild properties before types and the second export
// stops matching the first; rebuild in the stored order (which the group no
// longer carries) and the two lists interleave into a value no reader asked
// for.
func TestQuerySource_AMixedSourceRebuildsInOneCanonicalOrder(t *testing.T) {
	opts := typeRefOptions()

	// interleaved on purpose: property, type, property, type
	snap := querySnapshot("relid-dueDate", "typeid-page", "relid-status", "typeid-wine")
	data, err := Marshal(model.SmartBlockType_Page, snap, opts)
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (§11 I1)")
	assert.Contains(t, string(data), `"query_source": {
    "types": [
      "type-page",
      "type-wine"
    ],
    "properties": [
      "dueDate",
      "status"
    ]
  }`, "each list keeps its own stored order; the interleaving between them is what goes")

	sbType, back, err := Unmarshal(data, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"typeid-page", "typeid-wine", "relid-dueDate", "relid-status"},
		valueStringList(back.GetDetails().GetFields()["setOf"]),
		"types first, then properties — the canonical rebuild order (§11)")

	second, err := Marshal(sbType, back, opts)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(second),
		"the partition converges in one generation: every export after the first is byte-identical (§11)")
	_, back2, err := Unmarshal(second, opts)
	require.NoError(t, err)
	assert.Equal(t, valueStringList(back.GetDetails().GetFields()["setOf"]),
		valueStringList(back2.GetDetails().GetFields()["setOf"]),
		"and the stored value is a fixpoint after it")
}

// THREE STATES, not two — the Manifest.Files rule (§2c). A document that
// states no query has no group; a query that names no source has an EMPTY
// one; a query has a populated one. §6.2 already treats the middle case as
// its own thing, and one corpus dataview block names such a set.
//
// How this can fail: write the group with setNonEmpty and the middle state
// collapses into the first, which is the exact mistake Manifest.empty()
// records; drop the presence test and every document grows a group.
func TestQuerySource_StatesNoQuerySeparatelyFromAQueryNamingNothing(t *testing.T) {
	opts := typeRefOptions()

	t.Run("no stored key, no group", func(t *testing.T) {
		snap := querySnapshot()
		delete(snap.Details.Fields, "setOf")
		data, err := Marshal(model.SmartBlockType_Page, snap, opts)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "query_source")

		_, back, err := Unmarshal(data, opts)
		require.NoError(t, err)
		_, stated := back.GetDetails().GetFields()["setOf"]
		assert.False(t, stated, "and nothing is invented on the way back")
	})

	t.Run("the key present and empty is a query naming no source", func(t *testing.T) {
		data, err := Marshal(model.SmartBlockType_Page, querySnapshot(), opts)
		require.NoError(t, err)
		require.NoError(t, Validate(data, Options{}))
		assert.Contains(t, string(data), `"query_source": {}`,
			"an empty group states a query with no source — not the absence of a query")

		_, back, err := Unmarshal(data, opts)
		require.NoError(t, err)
		v, stated := back.GetDetails().GetFields()["setOf"]
		require.True(t, stated, "presence survives the round trip (§3)")
		assert.Empty(t, valueStringList(v))
	})
}

// The flat spelling is REFUSED, by Validate and by Unmarshal alike (§12 I2),
// with the repair named. Unlike §2a's and §2d's refusals this one is
// UNCONDITIONAL: there is no kind on which `setOf` in `properties` means
// something else.
//
// How this can fail: kind-scope the refusal and a set document keeps two
// legal spellings of its query; drop it and export writes one spelling while
// import accepts two.
func TestQuerySource_TheFlatSpellingIsRefusedOnEveryKind(t *testing.T) {
	for name, doc := range map[string]string{
		"a set":               `{"formatVersion":"2.0","id":"s1","properties":{"Set of":["type-task"]}}`,
		"a template":          `{"formatVersion":"2.0","kind":"template","type":"Template","template_for":"type-set","properties":{"Set of":["type-task"]}}`,
		"a relation":          `{"formatVersion":"2.0","kind":"property","internal_key":"k","property_settings":{"format":"text"},"properties":{"Set of":["type-task"]}}`,
		"the stored spelling": `{"formatVersion":"2.0","id":"s2","properties":{"setOf":["type-task"]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			err := Validate([]byte(doc), Options{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), `is written on the root as "query_source"`)
			_, _, importErr := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
			require.Error(t, importErr, "Validate and the import seam agree (§12 I2)")
			assert.Contains(t, importErr.Error(), `is written on the root as "query_source"`)
		})
	}
}

// An entry in the wrong list is refused where bytes can see it: a `type-`
// derived id in the PROPERTY list. The other direction cannot be checked —
// a bare stored key and a store id look alike — and that asymmetry is
// exactly why `types` states the derived id and `properties` does not.
//
// How this can fail: drop the prefix test and `{"properties":["type-task"]}`
// imports as a property whose stored key is the literal string `type-task`,
// which names nothing in any space.
func TestQuerySource_ATypeInThePropertyListIsRefused(t *testing.T) {
	doc := `{"formatVersion":"2.0","id":"s1","query_source":{"properties":["type-task"]}}`
	err := Validate([]byte(doc), Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wears the reserved type- prefix")
	assert.Contains(t, err.Error(), `a type target belongs in "query_source.types"`)

	_, _, importErr := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
	require.Error(t, importErr, "Validate and the import seam agree (§12 I2)")
	assert.Contains(t, importErr.Error(), "wears the reserved type- prefix")
}

// An entry no resolver can classify keeps its stored spelling, lands in
// `types`, and is REPORTED — the placement is the slot's declared domain
// speaking, not a finding. It round-trips exactly, which is what makes the
// fallback safe: 11 of the corpus's 174 values reach it, every one a
// tombstoned type.
//
// How this can fail: drop the entry instead of writing it and 11 corpus
// documents lose their whole query source; write it silently and nothing
// distinguishes a classified target from a guess.
func TestQuerySource_AnUnclassifiableTargetIsKeptInTypesAndReported(t *testing.T) {
	opts := typeRefOptions()
	var warnings []Issue
	opts.OnWarning = func(i Issue) { warnings = append(warnings, i) }

	data, err := Marshal(model.SmartBlockType_Page, querySnapshot("bafyreitombstoned"), opts)
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}))
	assert.Contains(t, string(data), `"query_source": {
    "types": [
      "bafyreitombstoned"
    ]
  }`)
	require.Len(t, warnings, 1)
	assert.Equal(t, "/query_source", warnings[0].Path)
	assert.Contains(t, warnings[0].Message, "names neither a type nor a property this export can resolve")

	_, back, err := Unmarshal(data, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"bafyreitombstoned"}, valueStringList(back.GetDetails().GetFields()["setOf"]),
		"an id nothing could classify is still the stored value's meaning and comes back untouched")
}

// A TYPE document states no query source. Its stored `setOf` is the type's
// OWN id, re-stamped by WithForcedDetail on every init, and §2a already
// drops it as install provenance — so the group asks that predicate rather
// than inventing a second kind test, and a document that states one anyway
// is refused on both surfaces (§12 I2).
//
// How this can fail: lift unconditionally without consulting the §2a drop
// and every type document grows a `query_source` naming itself.
func TestQuerySource_ATypeDocumentStatesNone(t *testing.T) {
	opts := typeRefOptions()
	snap := typeSnapshot()
	snap.Details.Fields["setOf"] = strList("typeObjectId")

	data, err := Marshal(model.SmartBlockType_STType, snap, opts)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "query_source",
		"§2a drops the key first, so there is nothing to lift")
	assert.NotContains(t, string(data), `"Set of"`)

	doc := `{"formatVersion":"2.0","kind":"object_type","id":"type-k","internal_key":"k",` +
		`"query_source":{"types":["type-k"]}}`
	err = Validate([]byte(doc), Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a type document states no query source")
	_, _, importErr := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
	require.Error(t, importErr, "Validate and the import seam agree (§12 I2)")
	assert.Contains(t, importErr.Error(), "a type document states no query source")
}

// `NoDerivedTypeIds` reaches `types` and not `properties`, and that split is
// the whole of the mode's answer here (§9). `types` is a type-KEY slot, so
// mode-on it carries the VOCABULARY spelling, the same word the envelope
// `type` writes — one type, one word, in every slot that names it as a kind.
// `properties` holds a stored key, which is not a derived id, so there is
// nothing for the mode to decline.
//
// How this can fail: route `types` through foldTypeRef and it keeps the
// store id mode-on, which is the reference-slot treatment and not this
// slot's; make `properties` mode-aware and the mode starts changing a
// spelling that was never derived.
func TestQuerySource_TheModeReachesTypesAndNotProperties(t *testing.T) {
	opts := noDerivedOptions()

	data, err := Marshal(model.SmartBlockType_Page,
		querySnapshot("typeid-page", "relid-dueDate"), opts)
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}))
	assert.NotContains(t, string(data), TypeRefPrefix, "mode-on, no slot writes a derived type id")
	assert.Contains(t, string(data), `"query_source": {
    "types": [
      "page"
    ],
    "properties": [
      "dueDate"
    ]
  }`, "the vocabulary's word for the type (here an api-like key), the stored key for the property")

	_, back, err := Unmarshal(data, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"typeid-page", "relid-dueDate"},
		valueStringList(back.GetDetails().GetFields()["setOf"]),
		"both spellings read back to the store ids the slot holds")
}

// A `query_source.properties` entry is a property USE, and the ONE place a
// property key appears with no spelling anywhere in the document. The
// dictionary is used-only (§2f), so a census that counted spellings alone
// would leave a minted property named as a query source out of
// `properties.json` — and then nothing in the bundle could say what the query
// ranges over.
//
// It is a STORED KEY, never a spelling: it skips the §3 ladder the way a
// type declaration's `internal_key` does.
//
// How this can fail: leave PropertyTermsOf reading spellings only and the
// dictionary silently shrinks by exactly the properties queries name.
func TestQuerySource_APropertyTargetIsAPropertyUse(t *testing.T) {
	terms, err := PropertyTermsOf([]byte(`{"formatVersion":"2.0","id":"s1",
		"query_source":{"types":["type-task"],"properties":["6a32d4856761631534b22f85","lastModifiedDate"]}}`))
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{
		"6a32d4856761631534b22f85": true,
		"lastModifiedDate":         true,
	}, terms.StoredKeys, "both entries are stored keys, and the type list is not a property use")
	assert.Empty(t, terms.Spellings, "a query source names no spelling")
}

// A stored value that is already a bare KEY rather than an id — the shape
// the sibling `object_types` slot carries on 21 production entries — still
// lands in the right list, through the same two capabilities asked the other
// way round.
//
// How this can fail: ask only the id half and a legacy key falls through to
// the unclassified arm, where it is written into `types` whatever it is —
// silently right half the time.
func TestQuerySource_ALegacyBareKeyLandsInTheRightList(t *testing.T) {
	opts := typeRefOptions()
	var warnings []Issue
	opts.OnWarning = func(i Issue) { warnings = append(warnings, i) }

	data, err := Marshal(model.SmartBlockType_Page, querySnapshot("page", "dueDate"), opts)
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}))
	assert.Contains(t, string(data), `"query_source": {
    "types": [
      "type-page"
    ],
    "properties": [
      "dueDate"
    ]
  }`)
	assert.Empty(t, warnings, "both resolved, so neither is a guess")
}

// A `types` entry wearing the reserved prefix whose tail is not a stored type
// key is refused where it stands, on both surfaces (§9, §12 I2) — the same
// refusal `template_for` and every `object_types` make. Falling through would
// hand `type-` to the vocabulary as a display SPELLING and look up a type
// named "type-".
//
// How this can fail: drop the semantic arm and Validate passes a document
// Unmarshal refuses, which is the two surfaces disagreeing about one
// document.
func TestQuerySource_AMalformedDerivedIdIsRefusedWhereItStands(t *testing.T) {
	doc := `{"formatVersion":"2.0","id":"s1","query_source":{"types":["type-"]}}`
	err := Validate([]byte(doc), Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/query_source/types/0")
	assert.Contains(t, err.Error(), "is not a stored type key")

	_, _, importErr := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
	require.Error(t, importErr, "Validate and the import seam agree (§12 I2)")
	assert.Contains(t, importErr.Error(), "is not a stored type key")
}

// A stored PROPERTY key that happens to wear the reserved `type-` prefix
// cannot be written into the property list, because Validate refuses an
// entry there that wears it — so writing one would hand back a document
// this package's own Validate rejects (§11 I1). The entry keeps the
// property's object id instead, which is what the stored slot held anyway,
// and the loss is reported.
//
// A stored key is not gated the way a type key is (`property_internal_keys`
// accepts any control-character-free string), so this is reachable from a
// real space and not only from a synthetic snapshot.
//
// How this can fail: write the key through unguarded and Marshal emits what
// Validate rejects — the one invariant the codec is held to against
// untrusted snapshots.
func TestQuerySource_APropertyKeyWearingTheReservedPrefixKeepsItsId(t *testing.T) {
	res := newTypeIdVocabulary()
	res.byId["relid-lookalike"] = PropertyDefinition{Key: "type-lookalike", Name: "Lookalike"}
	opts := foldOptions()
	opts.ResolveProperties = res
	var warnings []Issue
	opts.OnWarning = func(i Issue) { warnings = append(warnings, i) }

	data, err := Marshal(model.SmartBlockType_Page, querySnapshot("relid-lookalike"), opts)
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (§11 I1)")
	assert.Contains(t, string(data), `"properties": [
      "relid-lookalike"
    ]`, "the id stands in for a key the list may not hold")
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Message, "wears the reserved type- prefix")

	_, back, err := Unmarshal(data, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"relid-lookalike"}, valueStringList(back.GetDetails().GetFields()["setOf"]))
}

// The other half of the same I1 guard: a stored property key with no
// written form at all. Nothing gates a stored property key, so a space can
// hold one carrying a control character or running past the 128-rune bound
// the member names accept (§3) — and the import seam refuses such a key, so
// writing it would hand back a document this package cannot read.
//
// How this can fail: write the key through and Unmarshal refuses what
// Marshal produced.
func TestQuerySource_APropertyKeyWithNoWrittenFormKeepsItsId(t *testing.T) {
	res := newTypeIdVocabulary()
	res.byId["relid-unwritable"] = PropertyDefinition{Key: "carriage\rreturn", Name: "Unwritable"}
	opts := foldOptions()
	opts.ResolveProperties = res
	var warnings []Issue
	opts.OnWarning = func(i Issue) { warnings = append(warnings, i) }

	data, err := Marshal(model.SmartBlockType_Page, querySnapshot("relid-unwritable"), opts)
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (§11 I1)")
	assert.Contains(t, string(data), `"properties": [
      "relid-unwritable"
    ]`)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Message, "carries a control character")

	_, back, err := Unmarshal(data, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"relid-unwritable"}, valueStringList(back.GetDetails().GetFields()["setOf"]))
}
