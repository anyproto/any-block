package anyblockjson

// installedtypes_test.go pins the surface split the type planner draws (§2g):
// an AUTHOR minting a namespace and an EXPORT describing one a space already
// holds ask different questions of the same declarations, and only the first
// may refuse a collision.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An export carries a space's installed bundled types as ordinary
// `object_type` documents — `internal_key: "task"`, `Name: "Task"`, or the
// name the space renamed it to. That is byte-for-byte the shape an AUTHORED
// declaration would use to shadow the bundled Task, and the planner refused
// both alike.
func TestInstalledPlanAdmitsBundledTypeIdentities(t *testing.T) {
	documents := map[string][]byte{
		"types/type-task.anyblock.json": typeDeclarationDocument("Task", "task"),
		"types/type-page.anyblock.json": typeDeclarationDocument("Homework", "page"),
	}

	_, err := PlanAuthoringTypeVocabulary(documents, AuthoringVocabularyPlanOptions{})
	require.ErrorContains(t, err, `stored type key "task" conflicts with bundled type key(s) "task"`,
		"the authoring surface still refuses a declaration that shadows a bundled key")

	vocab, err := PlanAuthoringTypeVocabulary(documents, AuthoringVocabularyPlanOptions{Installed: true})
	require.NoError(t, err)

	assert.Equal(t, "Homework", vocab.TypeSlug("page"),
		"a renamed installed type is captioned by the space's name, not the shipped table's")
	assert.Equal(t, "Task", vocab.TypeSlug("task"))

	key, resolved := vocab.TypeKey("task")
	assert.False(t, resolved, "a live stored key is its own address, verbatim-first (§3)")
	assert.Equal(t, "task", key)

	key, resolved = vocab.TypeKey("Homework")
	assert.True(t, resolved)
	assert.Equal(t, "page", key)

	key, resolved = vocab.TypeKey("Task")
	assert.True(t, resolved)
	assert.Equal(t, "task", key, "the installed type answers to its own name once")
}

// Two live types of one space may share a caption; the space allows it and the
// export has to describe it. The planner records every claimant, and the
// ambiguity is refused where a slot actually has to resolve the caption.
func TestInstalledPlanKeepsEveryClaimantOfAContestedCaption(t *testing.T) {
	const first, second = "692de7b44c932bae256c957d", "69346f554c932bae256cbd02"
	documents := map[string][]byte{
		"types/type-" + first + ".anyblock.json":  typeDeclarationDocument("Event", first),
		"types/type-" + second + ".anyblock.json": typeDeclarationDocument("Event", second),
	}

	_, err := PlanAuthoringTypeVocabulary(documents, AuthoringVocabularyPlanOptions{})
	require.ErrorContains(t, err, `type display name "Event" is already claimed for stored type key`,
		"the authoring surface still refuses one author declaring two types under one caption")

	vocab, err := PlanAuthoringTypeVocabulary(documents, AuthoringVocabularyPlanOptions{Installed: true})
	require.NoError(t, err)

	assert.Equal(t, []string{first, second}, vocab.TypeKeyCandidates("Event"),
		"both claimants stay reachable; neither is dropped for arriving second")
	key, resolved := vocab.TypeKey("Event")
	assert.False(t, resolved)
	assert.Equal(t, "Event", key)

	_, _, err = Unmarshal([]byte(`{"formatVersion":"2.0","id":"one","type":"Event"}`), Options{Keys: vocab})
	require.ErrorContains(t, err, `the spelling "Event" names 2 live types in this space`,
		"the refusal moves to the slot that has to resolve the caption")

	_, _, err = Unmarshal([]byte(`{"formatVersion":"2.0","id":"one","type":"Event","type_internal_key":"`+first+`"}`),
		Options{Keys: vocab})
	require.NoError(t, err, "an exported document states its key beside the caption and needs no resolution (§2)")
}

// A space's own type may be captioned like a bundled one — the corpus holds
// Recipe, Page, Space and Goal. The bundled table is one more claimant, not a
// reservation.
func TestInstalledPlanTreatsABundledCaptionAsOneMoreClaimant(t *testing.T) {
	const key = "692473c24c932babe6b32354"
	documents := map[string][]byte{
		"types/type-" + key + ".anyblock.json": typeDeclarationDocument("Recipe", key),
	}

	_, err := PlanAuthoringTypeVocabulary(documents, AuthoringVocabularyPlanOptions{})
	require.ErrorContains(t, err, `type display name "Recipe" conflicts with bundled type key(s) "recipe"`,
		"the authoring surface still refuses a caption that shadows a bundled type")

	vocab, err := PlanAuthoringTypeVocabulary(documents, AuthoringVocabularyPlanOptions{Installed: true})
	require.NoError(t, err)

	assert.Equal(t, []string{key, "recipe"}, vocab.TypeKeyCandidates("Recipe"))
	_, _, err = Unmarshal([]byte(`{"formatVersion":"2.0","id":"one","type":"Recipe"}`), Options{Keys: vocab})
	require.ErrorContains(t, err, `the spelling "Recipe" names 2 live types in this space`)
}

// A caption may also be spelled exactly like ANOTHER live type's stored key.
// Verbatim-first (§3) already settles that: the key answers for itself and the
// caption joins as a claim the stored address outranks.
func TestInstalledPlanLetsACaptionStandBesideALiveStoredKey(t *testing.T) {
	documents := map[string][]byte{
		"types/a.json": typeDeclarationDocument("Ritual", "ritual_v2"),
		"types/b.json": typeDeclarationDocument("ritual_v2", "68c2a23c96ab900e02935111"),
	}

	_, err := PlanAuthoringTypeVocabulary(documents, AuthoringVocabularyPlanOptions{})
	require.ErrorContains(t, err,
		`type display name "ritual_v2" conflicts with live stored type key declared by types/a.json`,
		"the authoring surface still refuses a caption spelled like a live stored key")

	vocab, err := PlanAuthoringTypeVocabulary(documents, AuthoringVocabularyPlanOptions{Installed: true})
	require.NoError(t, err)

	key, resolved := vocab.TypeKey("ritual_v2")
	assert.False(t, resolved)
	assert.Equal(t, "ritual_v2", key, "a live stored key outranks a caption spelled like it")
	assert.Equal(t, []string{"68c2a23c96ab900e02935111"}, vocab.TypeKeyCandidates("ritual_v2"),
		"the caption is still a candidate, so a wider reader can see the contest")
}

// Relaxing the collision refusals must not relax the one rule that is a defect
// on BOTH surfaces: two documents cannot define one stored key, because a type
// document's id is a pure function of that key (§9).
func TestInstalledPlanStillRefusesTwoDeclarationsOfOneStoredKey(t *testing.T) {
	documents := map[string][]byte{
		"types/a.json": typeDeclarationDocument("First", "custom_key"),
		"types/b.json": typeDeclarationDocument("Second", "custom_key"),
	}
	for name, opts := range map[string]AuthoringVocabularyPlanOptions{
		"authoring": {},
		"installed": {Installed: true},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := PlanAuthoringTypeVocabulary(documents, opts)
			require.ErrorContains(t, err, `stored type key "custom_key" is already declared by types/a.json`)
		})
	}
}
