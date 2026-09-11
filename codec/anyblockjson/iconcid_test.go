package anyblockjson

// iconcid_test.go pins the icon's fifth variant (§2b): an image addressed by
// its CONTENT cid, with no file object behind it — a participant's avatar, a
// 1-to-1 space's icon — fetched through the gateway or the identity repo.
// `file` used to carry both meanings and let a reader tell them apart by
// decoding the CID's codec; a slot with two meanings is a defect, so the
// content address gets its own member and the codec decides which one an
// export writes: dag-cbor is an object id, dag-pb or raw is content.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/internal/testfixtures"
)

func TestExport_AContentCidIconIsItsOwnVariant(t *testing.T) {
	t.Run("a dag-pb cid is content, not an object", func(t *testing.T) {
		icon, _, _, _ := exportedIconCover(t, map[string]*types.Value{
			"iconImage": strList(testfixtures.ContentID), "iconOption": num(5)})
		assert.Equal(t, `{"format":"cid","cid":"`+testfixtures.ContentID+`","color":"pink"}`, compactJSON(t, icon))
	})
	t.Run("a dag-cbor cid is still a file object reference", func(t *testing.T) {
		icon, _, _, _ := exportedIconCover(t, map[string]*types.Value{"iconImage": strList(testfixtures.ObjectID)})
		assert.Equal(t, `{"format":"file","file":"`+testfixtures.ObjectID+`"}`, compactJSON(t, icon))
	})
	t.Run("it imports back onto the same slot and is a fixpoint", func(t *testing.T) {
		doc := `{"formatVersion":"2.0","id":"o1","icon":{"format":"cid","cid":"` + testfixtures.ContentID + `"}}`
		require.NoError(t, Validate([]byte(doc), Options{}))
		_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.NoError(t, err)
		assert.Equal(t, strList(testfixtures.ContentID), snap.Details.Fields[detailKeyIconImage])
		icon, _, _, _ := exportedIconCover(t, map[string]*types.Value{"iconImage": strList(testfixtures.ContentID)})
		assert.Equal(t, `{"format":"cid","cid":"`+testfixtures.ContentID+`"}`, compactJSON(t, icon))
	})
	t.Run("an author cannot mint a content address", func(t *testing.T) {
		doc := `{"formatVersion":"2.0","id":"o1","type":"Page","icon":{"format":"cid","cid":"` + testfixtures.ContentID + `"}}`
		require.Error(t, ValidateAuthoring([]byte(doc)))
	})
}

func TestIndex_AContentCidIconNamesNoObject(t *testing.T) {
	t.Run("the variant round-trips through the index", func(t *testing.T) {
		data, err := MarshalIndex(&Index{Name: "Corpus", Icon: &Icon{Format: "cid", Cid: testfixtures.ContentID}}, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"format": "cid"`)
		back, err := UnmarshalIndex(data, Options{})
		require.NoError(t, err)
		require.NotNil(t, back.Icon)
		assert.Equal(t, testfixtures.ContentID, back.Icon.Cid)
		assert.Empty(t, back.IconImageId(), "a content cid names no object the bundle could carry")
	})
	t.Run("the legacy overloaded spelling is input compatibility, and names no object either", func(t *testing.T) {
		legacy := &Index{Name: "Corpus", Icon: &Icon{Format: "file", File: testfixtures.ContentID}}
		assert.Empty(t, legacy.IconImageId())
		object := &Index{Name: "Corpus", Icon: &Icon{Format: "file", File: testfixtures.ObjectID}}
		assert.Equal(t, testfixtures.ObjectID, object.IconImageId())
	})
}
