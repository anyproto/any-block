package anyblockjson

// optioncollision_test.go — two options of ONE property sharing ONE name, and
// the same object touching both (§3, §9a).
//
// The legend is `{property spelling: {option name: option id}}`, keyed by
// NAME, so one document has room for exactly one entry per name per property.
// An object sitting on both same-named options therefore had nowhere to put
// the second id, and both values collapsed onto the first on reimport — the
// object lost a tag. Measured on the 24 905-document corpus at out-57f4add:
// one document, "Read Write Own: Building the Next Era of the Internet",
// spells `"Tag": ["books","books","book","read"]` where the two `books` are
// different options (red `663acb5a…370c`, yellow `663acb4c…370a`).
//
// The rule is the collision discipline the format already applies to property
// spellings (§3): census the option ids the document writes under one
// property, and where two of them claim ONE name, degrade EVERY claimant —
// both, never just the loser, so the written term does not depend on which
// slot claimed first.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// The corpus shape, reduced: one object on BOTH same-named options.
func TestOptionCollision_OneObjectOnBothSameNamedOptions(t *testing.T) {
	// given — the pool lists "books" twice, and the object is on both
	space := spaceOptions{"tag": {
		{id: "bafyfirst", name: "books"},
		{id: "bafysecond", name: "books"},
		{id: "bafythird", name: "read"},
	}}
	snap := optionSnapshot(map[string]*types.Value{
		"tag": strList("bafysecond", "bafyfirst", "bafythird"),
	})
	opts := Options{ResolveOptions: space}

	// when
	data, err := Marshal(model.SmartBlockType_Page, snap, opts)
	require.NoError(t, err)

	// then — both claimants degrade, the uncontested name is untouched, and
	// the legend has one entry per option rather than one per name
	require.NoError(t, Validate(data, Options{}))
	assert.Equal(t, []any{"books (second)", "books (yfirst)", "read"},
		docProperty(t, data, "Tag"))
	assert.Equal(t, legend("Tag", map[string]string{
		"books (second)": "bafysecond",
		"books (yfirst)": "bafyfirst",
		"read":           "bafythird",
	}), docOptionIds(t, data))

	// and the object keeps both options on the way back
	_, back, err := Unmarshal(data, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"bafysecond", "bafyfirst", "bafythird"},
		storedList(t, back, "tag"))

	// and it is a FIXPOINT: exporting what came back reproduces the document
	again, err := Marshal(model.SmartBlockType_Page, back, opts)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(again))
}

// The census is over the PROPERTY, not over one slot: a filter value and a
// property value are two ways for one document to claim one option name, and
// a census that only looked at the value list would leave the legend one
// entry short exactly as before.
func TestOptionCollision_ACrossSlotPairDegradesToo(t *testing.T) {
	// given — the property value holds one "books", a filter holds the other
	space := spaceOptions{"tag": {
		{id: "bafyfirst", name: "books"},
		{id: "bafysecond", name: "books"},
	}}
	dv := &model.BlockContentDataview{
		RelationLinks: []*model.RelationLink{{Key: "tag", Format: model.RelationFormat_tag}},
		Views: []*model.BlockContentDataviewView{{Id: "v1", Name: "All",
			Filters: []*model.BlockContentDataviewFilter{{
				Id: "f1", RelationKey: "tag", Format: model.RelationFormat_tag,
				Condition: model.BlockContentDataviewFilter_In,
				Value:     strList("bafysecond"),
			}},
		}},
	}
	snap := &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{
			{Id: "obj1", ChildrenIds: []string{"dv1"},
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
			{Id: "dv1", Content: &model.BlockContentOfDataview{Dataview: dv}},
		},
		Details: fields(map[string]*types.Value{
			"id": str("bafyreiticket"), "name": str("Board"),
			"tag": strList("bafyfirst"),
		}),
	}
	opts := Options{ResolveOptions: space}

	// when
	data, err := Marshal(model.SmartBlockType_Page, snap, opts)
	require.NoError(t, err)

	// then
	require.NoError(t, Validate(data, Options{}))
	assert.Equal(t, []any{"books (yfirst)"}, docProperty(t, data, "Tag"))
	assert.Equal(t, legend("Tag", map[string]string{
		"books (yfirst)": "bafyfirst",
		"books (second)": "bafysecond",
	}), docOptionIds(t, data))

	_, back, err := Unmarshal(data, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"bafyfirst"}, storedList(t, back, "tag"))
	assert.Equal(t, []string{"bafysecond"}, dvFilterValues(t, back))
}

// The value-level surface carries the same legend (MarshalPropertyValue) and
// so it owes the same census: a caller handed `["books","books"]` beside a
// one-entry map is holding the loss this rule removes.
func TestOptionCollision_TheValueLevelSurfaceCensusesToo(t *testing.T) {
	// given
	space := spaceOptions{"tag": {
		{id: "bafyfirst", name: "books"},
		{id: "bafysecond", name: "books"},
	}}

	// when
	value, ids := MarshalPropertyValue("tag", strList("bafysecond", "bafyfirst"),
		Options{ResolveOptions: space})

	// then
	assert.Equal(t, []any{"books (second)", "books (yfirst)"}, value)
	assert.Equal(t, map[string]string{
		"books (second)": "bafysecond",
		"books (yfirst)": "bafyfirst",
	}, ids)
}

// A name only ONE option of the property claims is written plainly, whatever
// else the pool holds: the census is a fact about the DOCUMENT, and degrading
// an uncontested claimant would cost every reader a readable name for
// nothing. The document carries two options here on purpose — a census of one
// short-circuits before the contest is ever counted, so a one-option document
// would pass this test without asking the question.
func TestOptionCollision_ASoleClaimantKeepsItsName(t *testing.T) {
	// given — the pool holds two "books", and the object is on one of them
	// plus an option nothing contests
	space := spaceOptions{"tag": {
		{id: "bafyfirst", name: "books"},
		{id: "bafysecond", name: "books"},
		{id: "bafythird", name: "read"},
	}}
	snap := optionSnapshot(map[string]*types.Value{"tag": strList("bafysecond", "bafythird")})
	opts := Options{ResolveOptions: space}

	// when
	data, err := Marshal(model.SmartBlockType_Page, snap, opts)
	require.NoError(t, err)

	// then
	assert.Equal(t, []any{"books", "read"}, docProperty(t, data, "Tag"))
	assert.Equal(t, legend("Tag", map[string]string{
		"books": "bafysecond",
		"read":  "bafythird",
	}), docOptionIds(t, data))
}

// Rung (c), reached two ways, and both send EVERY affected claimant to the
// bare id rather than letting one of them take a term another option answers
// to. It is the residue the ladder cannot spell, and it is written down here
// so that the price — a select value a reader cannot render — is a decision
// rather than a discovery.
func TestOptionCollision_TheUnspellableResidueFallsBackToTheId(t *testing.T) {
	for name, tc := range map[string]struct {
		space spaceOptions
		ids   []string
		want  []any
		// legend entries the degraded pair owes; a term that IS the id says
		// nothing an entry could add, so the pair owes none
		wantLegend map[string]string
	}{
		// two options with one name whose ids share their last six
		// characters: one suffixed form, two claimants, so neither may have
		// it
		"a residual tie on name and tail": {
			space: spaceOptions{"tag": {
				{id: "bafya-shared", name: "books"},
				{id: "bafyb-shared", name: "books"},
			}},
			ids:        []string{"bafya-shared", "bafyb-shared"},
			want:       []any{"bafya-shared", "bafyb-shared"},
			wantLegend: nil,
		},
		// a THIRD option of the same property is literally named the
		// suffixed form one claimant would take, so that claimant falls
		// through and the other keeps its rung (b)
		"a suffix another option already answers to": {
			space: spaceOptions{"tag": {
				{id: "bafyfirst", name: "books"},
				{id: "bafysecond", name: "books"},
				{id: "bafythird", name: "books (yfirst)"},
			}},
			ids:  []string{"bafyfirst", "bafysecond", "bafythird"},
			want: []any{"bafyfirst", "books (second)", "books (yfirst)"},
			wantLegend: map[string]string{
				"books (second)": "bafysecond",
				"books (yfirst)": "bafythird",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			// given
			snap := optionSnapshot(map[string]*types.Value{"tag": strList(tc.ids...)})
			opts := Options{ResolveOptions: tc.space}

			// when
			data, err := Marshal(model.SmartBlockType_Page, snap, opts)
			require.NoError(t, err)

			// then
			require.NoError(t, Validate(data, Options{}))
			assert.Equal(t, tc.want, docProperty(t, data, "Tag"))
			if tc.wantLegend == nil {
				assert.Nil(t, docOptionIds(t, data))
			} else {
				assert.Equal(t, legend("Tag", tc.wantLegend), docOptionIds(t, data))
			}

			// and every option the object was on is still the option it comes
			// back on, which is the whole point of the fallback
			_, back, err := Unmarshal(data, opts)
			require.NoError(t, err)
			assert.Equal(t, tc.ids, storedList(t, back, "tag"))

			// and it is a fixpoint
			again, err := Marshal(model.SmartBlockType_Page, back, opts)
			require.NoError(t, err)
			assert.Equal(t, string(data), string(again))
		})
	}
}

// The census mirrors the EMIT, not the store: a detail key the property emit
// drops writes no value, so the option ids stored under it claim no name and
// contest nothing. Counting them would degrade a rival that had none — and
// break the fixpoint, because the next generation no longer holds the dropped
// key at all and spells the rival plainly (the fault seedTermLedger documents
// for property keys, one namespace over).
//
// `isNew` is the shape that makes this reachable: stripped from `properties`
// (§3), unknown to the bundled format table, so a caller's resolver is free
// to call it a select — and a dataview filter on it still reaches the emit.
func TestOptionCollision_ADroppedDetailKeyCensusesNothing(t *testing.T) {
	// given — the stored value holds BOTH same-named options and is dropped;
	// only the filter's one id is ever written
	const key = "isNew"
	space := spaceOptions{key: {
		{id: "bafyfirst", name: "books"},
		{id: "bafysecond", name: "books"},
	}}
	dv := &model.BlockContentDataview{
		RelationLinks: []*model.RelationLink{{Key: key, Format: model.RelationFormat_tag}},
		Views: []*model.BlockContentDataviewView{{Id: "v1", Name: "All",
			Filters: []*model.BlockContentDataviewFilter{{
				Id: "f1", RelationKey: key, Format: model.RelationFormat_tag,
				Condition: model.BlockContentDataviewFilter_In,
				Value:     strList("bafyfirst"),
			}},
		}},
	}
	snap := &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{
			{Id: "obj1", ChildrenIds: []string{"dv1"},
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
			{Id: "dv1", Content: &model.BlockContentOfDataview{Dataview: dv}},
		},
		Details: fields(map[string]*types.Value{
			"id": str("bafyreiticket"), "name": str("Board"),
			key: strList("bafyfirst", "bafysecond"),
		}),
	}
	opts := Options{ResolveOptions: space, ResolveFormat: selectFormats}

	// when
	data, err := Marshal(model.SmartBlockType_Page, snap, opts)
	require.NoError(t, err)

	// then — the sole WRITTEN claimant keeps its name
	require.NoError(t, Validate(data, Options{}))
	assert.Nil(t, docProperty(t, data, key), "the stored value is dropped (§3)")
	assert.Equal(t, legend(key, map[string]string{"books": "bafyfirst"}), docOptionIds(t, data))

	// and it is a fixpoint, which is the half a stored-value census breaks:
	// the second generation does not hold the dropped key at all
	_, back, err := Unmarshal(data, opts)
	require.NoError(t, err)
	again, err := Marshal(model.SmartBlockType_Page, back, opts)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(again))
}

// dvFilterValues reads the first view's first filter value off an imported
// snapshot, as a string list.
func dvFilterValues(t *testing.T, snap *model.SmartBlockSnapshotBase) []string {
	t.Helper()
	for _, b := range snap.Blocks {
		c, ok := b.Content.(*model.BlockContentOfDataview)
		if !ok {
			continue
		}
		require.NotEmpty(t, c.Dataview.Views)
		require.NotEmpty(t, c.Dataview.Views[0].Filters)
		return valueStringList(c.Dataview.Views[0].Filters[0].Value)
	}
	t.Fatal("no dataview block")
	return nil
}

// The avoid-set is the PROPERTY's options, not the ones this document
// happens to census. §3 says the bare id is written where the suffixed form
// is "a form some other option of the property is already named", and a
// document is free to write two of three same-named options: the third never
// enters the census, so a document-scoped check cannot see the name it
// already answers to.
//
// What that costs is the exact fault this rule exists to prevent. The term
// `books (yfirst)` is minted for one option while a DIFFERENT live option is
// literally named it, so a reader that cannot use the id — a bundle installed
// into a space that never saw it — resolves the term by name and lands the
// object on an option it was never on.
func TestOptionCollision_TheAvoidSetIsThePropertysOptions(t *testing.T) {
	// given — three options, and the document is on two of them
	space := spaceOptions{"tag": {
		{id: "bafyfirst", name: "books"},
		{id: "bafysecond", name: "books"},
		{id: "bafythird", name: "books (yfirst)"},
	}}
	snap := optionSnapshot(map[string]*types.Value{"tag": strList("bafyfirst", "bafysecond")})
	opts := Options{ResolveOptions: space}

	// when
	data, err := Marshal(model.SmartBlockType_Page, snap, opts)
	require.NoError(t, err)

	// then — the claimant whose suffixed form `bafythird` already answers to
	// falls to rung (c); the other keeps rung (b)
	require.NoError(t, Validate(data, Options{}))
	assert.Equal(t, []any{"bafyfirst", "books (second)"}, docProperty(t, data, "Tag"))
	assert.Equal(t, legend("Tag", map[string]string{"books (second)": "bafysecond"}),
		docOptionIds(t, data))

	// and the term no longer names the wrong option in a space that cannot
	// use the ids: the same three options, minted independently
	target := spaceOptions{"tag": {
		{id: "tgt1", name: "books"},
		{id: "tgt2", name: "books"},
		{id: "tgt3", name: "books (yfirst)"},
	}}
	_, back, err := Unmarshal(data, Options{ResolveOptions: target})
	require.NoError(t, err)
	assert.NotContains(t, storedList(t, back, "tag"), "tgt3",
		"no written term may resolve to an option the object was never on")
}

// `OmitIds` drops the legend (§9), and the degraded term exists only to give
// that legend room for a second entry. Written without it the term names
// nothing anywhere: its six characters are a fragment of an id the document
// no longer carries, and the reading side has neither an id to check nor an
// option of that name to find, so the wiring mints an option LITERALLY called
// `books (yfirst)`.
//
// That is strictly worse than the behaviour this rule replaced, which wrote
// `["books","books"]` and landed both on a real option. So the plan is gated
// on the legend it exists to serve.
func TestOptionCollision_OmitIdsWritesNoDegradedTerm(t *testing.T) {
	// given
	space := spaceOptions{"tag": {
		{id: "bafyfirst", name: "books"},
		{id: "bafysecond", name: "books"},
	}}
	snap := optionSnapshot(map[string]*types.Value{"tag": strList("bafyfirst", "bafysecond")})

	// when
	data, err := Marshal(model.SmartBlockType_Page, snap, Options{ResolveOptions: space, OmitIds: true})
	require.NoError(t, err)

	// then — the plain name, and no legend
	require.NoError(t, Validate(data, Options{}))
	assert.Equal(t, []any{"books", "books"}, docProperty(t, data, "Tag"))
	assert.Nil(t, docOptionIds(t, data))

	// and the values come back on the real option rather than minting two
	// synthetic names — the identity loss OmitIds already accepts, not a new
	// one it does not
	_, back, err := Unmarshal(data, Options{ResolveOptions: space})
	require.NoError(t, err)
	assert.Equal(t, []string{"bafyfirst", "bafyfirst"}, storedList(t, back, "tag"))
}

// A degraded term is only as good as the legend beside it, and §3 already
// says what happens when the legend cannot answer: "the name resolves exactly
// as it did before the legend existed". For a degraded term it did not — the
// space knows no option called `books (yfirst)`, so resolution fell straight
// through to the value unchanged and the wiring minted the synthetic name.
//
// That is the case the whole naming rule is FOR: §3 spells options by name
// because "a bundle carries no option objects", so the install that reads
// them is exactly the one whose space never saw the ids.
func TestOptionCollision_ADegradedTermResolvesByItsStem(t *testing.T) {
	// given — an export from a space holding two options named `books`
	source := spaceOptions{"tag": {
		{id: "bafyfirst", name: "books"},
		{id: "bafysecond", name: "books"},
	}}
	snap := optionSnapshot(map[string]*types.Value{"tag": strList("bafyfirst", "bafysecond")})
	data, err := Marshal(model.SmartBlockType_Page, snap, Options{ResolveOptions: source})
	require.NoError(t, err)
	require.Equal(t, []any{"books (yfirst)", "books (second)"}, docProperty(t, data, "Tag"))

	for name, tc := range map[string]struct {
		target spaceOptions
		want   []string
	}{
		// the install §3 is written for: none of the ids is live here, and
		// the target's own `books` is what the names mean
		"a space that has the option under that name": {
			target: spaceOptions{"tag": {{id: "bafytarget", name: "books"}}},
			want:   []string{"bafytarget", "bafytarget"},
		},
		// and where the name resolves to nothing either, what passes through
		// is the NAME — so the wiring mints one option called `books`, not
		// two called `books (yfirst)` and `books (second)`
		"a space that has no such option at all": {
			target: spaceOptions{"tag": {{id: "bafyother", name: "films"}}},
			want:   []string{"books", "books"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			// when
			_, back, err := Unmarshal(data, Options{ResolveOptions: tc.target})
			require.NoError(t, err)

			// then
			assert.Equal(t, tc.want, storedList(t, back, "tag"))
		})
	}
}

// The strip is the LAST name question asked, never the first, so an option a
// space really does name `<stem> (<six characters>)` is found under its own
// name and never mangled into its stem. One option name in the 2,490 the
// 79-bundle corpus at out-57f4add carries has the shape — `Other (logseq)` —
// and this is the rule that keeps it a name.
func TestOptionCollision_AnOptionNamedLikeADegradedTermResolvesToItself(t *testing.T) {
	// given — a document written by a space where that IS the option's name
	source := spaceOptions{"tag": {{id: "bafysource", name: "Other (logseq)"}}}
	snap := optionSnapshot(map[string]*types.Value{"tag": strList("bafysource")})
	data, err := Marshal(model.SmartBlockType_Page, snap, Options{ResolveOptions: source})
	require.NoError(t, err)
	require.Equal(t, []any{"Other (logseq)"}, docProperty(t, data, "Tag"))

	// when — a space that holds the same option under a different id, beside
	// one named the stem
	target := spaceOptions{"tag": {
		{id: "bafyother", name: "Other"},
		{id: "bafyexact", name: "Other (logseq)"},
	}}
	_, back, err := Unmarshal(data, Options{ResolveOptions: target})
	require.NoError(t, err)

	// then — the exact name wins; the stem is never consulted
	assert.Equal(t, []string{"bafyexact"}, storedList(t, back, "tag"))
}

// A reader with no option resolver has no space in which to ask any of §3's
// questions, and the strip is one of them: nothing in the string says whether
// `Other (logseq)` is a §3 term or an option's own name, and a reader with no
// vocabulary to check it against may not guess. §3 says such a reader changes
// nothing, and that is what keeps the unwired round trip byte-stable.
func TestOptionCollision_AReaderWithNoResolverStripsNothing(t *testing.T) {
	// given — a document written by a space that had two options named `books`
	source := spaceOptions{"tag": {
		{id: "bafyfirst", name: "books"},
		{id: "bafysecond", name: "books"},
	}}
	snap := optionSnapshot(map[string]*types.Value{"tag": strList("bafyfirst", "bafysecond")})
	data, err := Marshal(model.SmartBlockType_Page, snap, Options{ResolveOptions: source})
	require.NoError(t, err)

	// when — read with no resolver at all
	sbType, back, err := Unmarshal(data, Options{})
	require.NoError(t, err)

	// then — the terms survive as written
	assert.Equal(t, []string{"books (yfirst)", "books (second)"}, storedList(t, back, "tag"))

	// and the unwired round trip is a fixpoint: an unwired export writes back
	// exactly what it read
	again, err := Marshal(sbType, back, Options{})
	require.NoError(t, err)
	assert.Equal(t, []any{"books (yfirst)", "books (second)"}, docProperty(t, again, "Tag"))
}

// The prose half. A reader outside this repository holds the export and the
// published statements and nothing else, so those statements ARE the
// implementation — and four of them described a resolution a degraded term
// does not get. §3's algorithm said the inner key is "the option name exactly
// as the value spells it"; its step 2 resolved names "as before", which for a
// degraded term misses every time; and §2's envelope row and the published
// schema both promised that an id the space cannot honour leaves "the name"
// resolving as it would without the legend, which for a degraded term it did
// not. Each is pinned here because the freeze makes a wrong sentence
// permanent.
func TestOptionCollision_ThePublishedRulesResolveADegradedTerm(t *testing.T) {
	spec := readFormatDocumentation(t)["SPEC.md"]

	// §3's chain is the part an implementor codes.
	assert.NotContains(t, spec, "**Reading one option value: three steps",
		"the chain has a step for the term the degrade writes")
	assert.Contains(t, spec, "**Reading one option value: four steps, first answer wins.**")
	assert.Contains(t, spec, "3. **Name resolution again, on the name inside a degraded term.**",
		"a degraded term must resolve by its name rather than mint one")
	assert.NotContains(t, spec, "the inner\nkey the option **name** exactly as the value spells it",
		"the inner key is the TERM the slot writes, which for a degraded value is not a name")

	// §2's envelope row states the fallback a reader is promised.
	assert.Contains(t, spec,
		"a degraded term by the name inside it, which §3's step 3 strips the suffix to find",
		"§2 must state the fallback a degraded term actually gets")

	// and so does the published schema, in the same words.
	data, err := os.ReadFile(filepath.Join("..", "..", "format", "v2", "schema", "object.schema.json"))
	require.NoError(t, err)
	var schema struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(data, &schema))
	optionIds := schema.Properties["option_ids"].Description
	require.NotEmpty(t, optionIds)
	assert.NotContains(t, optionIds, "otherwise the name resolves as it would without the legend",
		"the schema promised a degraded term a resolution it did not get")
	assert.Contains(t, optionIds, "a degraded term by the name inside it",
		"the schema must state the same fallback §2 and §3 do")

	// §11 owns the trade the fallback makes, which nothing stated before.
	assert.Contains(t, spec, "In a space that never held those ids the legend answers nothing",
		"§11 must state what a cross-space install of a degraded term does")
}
