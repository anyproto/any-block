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
