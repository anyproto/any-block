package main

import (
	"archive/zip"
	"bytes"
	bundleconvert "github.com/anyproto/any-block/bundle/convert"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	env "github.com/anyproto/any-block/codec/anyblockjson/envelope"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bundleProfile = bundleconvert.Profile

func authoringBundleFixture() fstest.MapFS {
	documents := map[string]string{
		"index.json": `{"formatVersion":"2.0","name":"Weekends","entrypoint":"home","widgets":[{"target":"home"},{"target":"type-trip","layout":"view","limit":6},{"target":"_recent_open","layout":"compact_list","limit":4}]}`,
		"properties.json": `{"formatVersion":"2.0","properties":[
   {"name":"Budget","format":"number"},
   {"name":"State","format":"select","options":[{"name":"Booked","color":"blue"},{"name":"Dream","color":"yellow"}]},
   {"name":"Cost state","format":"select","options":["Booked"]},
   {"name":"When","format":"date","include_time":false},
   {"name":"Expenses","format":"objects","object_types":["Expense"]},
   {"name":"Trip","format":"objects","object_types":["Trip"]},
   {"name":"Tags","format":"multi_select","options":[{"name":"Art","color":"purple"},"Water"]}
  ]}`,
		"types/trip.json":      `{"formatVersion":"2.0","kind":"object_type","id":"type-trip","internal_key":"trip","properties":{"Name":"Trip"},"type_settings":{"default_template":"template-trip","property_definitions":[{"property":"Name"},{"property":"Budget","section":"featured"},{"property":"State"},{"property":"When"},{"property":"Expenses"},{"property":"Tags"}]},"blocks":[{"type":"paragraph","text":"Plan a weekend."}]}`,
		"types/expense.json":   `{"formatVersion":"2.0","kind":"object_type","id":"type-expense","internal_key":"expense","properties":{"Name":"Expense"},"type_settings":{"property_definitions":[{"property":"Cost state"},{"property":"Trip"}]}}`,
		"templates/trip.json":  `{"formatVersion":"2.0","kind":"template","type":"template","template_for":"Trip","id":"template-trip","properties":{"Name":"New weekend","State":"Booked","When":"2026-09-18T00:00:00Z"},"blocks":[{"type":"paragraph","text":"Pick a destination."}]}`,
		"objects/home.json":    `{"formatVersion":"2.0","id":"home","type":"Query","properties":{"Name":"Weekends"},"query_source":{"types":["Trip"],"properties":["Budget"]},"blocks":[{"type":"dataview","properties":[{"property":"State","format":"select"},{"property":"Budget","format":"number"}],"views":[{"name":"Booked weekends","type":"table","columns":[{"property":"State"},{"property":"Budget"}],"filters":[{"property":"State","condition":"in","value":["Booked"]}]}]}]}`,
		"objects/trip.json":    `{"formatVersion":"2.0","id":"weekend","type":"Trip","properties":{"Name":"Prague","Budget":750.5,"State":"Booked","When":"2026-09-18T00:00:00Z","Expenses":["hotel"],"Tags":["Art","Music"]},"icon":{"format":"emoji","emoji":"🚆"},"cover":{"format":"gradient","gradient":"sky"},"blocks":[{"type":"heading_2","text":"Saturday"},{"type":"paragraph","text":"Walk across Charles Bridge."},{"type":"link","object_id":"hotel"}]}`,
		"objects/expense.json": `{"formatVersion":"2.0","id":"hotel","type":"Expense","properties":{"Name":"Two hotel nights","Cost state":"Booked","Trip":["weekend"]},"blocks":[{"type":"paragraph","text":"A room for two near the old town."}]}`,
	}
	out := fstest.MapFS{}
	for name, data := range documents {
		out[name] = &fstest.MapFile{Data: []byte(data), Mode: 0o644}
	}
	return out
}

func writeAuthoringFixture(t *testing.T, fixture fstest.MapFS) string {
	t.Helper()
	dir := t.TempDir()
	for name, file := range fixture {
		require.NoError(t, writeOutput(filepath.Join(dir, filepath.FromSlash(name)), file.Data))
	}
	return dir
}

func decodeBundleSnapshots(t *testing.T, entries map[string][]byte, encoding string) map[string]*env.SnapshotWithType {
	t.Helper()
	out := map[string]*env.SnapshotWithType{}
	for name, data := range entries {
		if name == "profile" {
			continue
		}
		wrapper := &env.SnapshotWithType{}
		if encoding == "pb" {
			require.NoError(t, proto.Unmarshal(data, wrapper))
		} else {
			require.NoError(t, jsonpb.Unmarshal(bytes.NewReader(data), wrapper))
		}
		require.NotNil(t, wrapper.Snapshot)
		require.NotNil(t, wrapper.Snapshot.Data)
		id := wrapper.Snapshot.Data.Details.Fields["id"].GetStringValue()
		require.NotEmpty(t, id)
		require.NotContains(t, out, id)
		out[id] = wrapper
	}
	return out
}

func TestToV1AuthoringBundle(t *testing.T) {
	for _, tc := range []struct {
		encoding string
		zip      bool
	}{{"pb", true}, {"json", false}} {
		t.Run(tc.encoding, func(t *testing.T) {
			warnings := captureCLIWarnings(t)
			input := writeAuthoringFixture(t, authoringBundleFixture())
			output := filepath.Join(t.TempDir(), "converted")
			args := []string{"to-v1", "-in", input, "-out", output, "-encoding", tc.encoding, "-space-id", testSpaceID}
			if tc.zip {
				args = append(args, "-zip")
			}
			require.NoError(t, run(args))
			assert.NotContains(t, warnings.String(), "folded")
			entries := map[string][]byte{}
			if tc.zip {
				archive, err := zip.OpenReader(output)
				require.NoError(t, err)
				defer archive.Close()
				for _, file := range archive.File {
					reader, err := file.Open()
					require.NoError(t, err)
					data, err := io.ReadAll(reader)
					require.NoError(t, err)
					require.NoError(t, reader.Close())
					entries[file.Name] = data
				}
			} else {
				require.NoError(t, filepath.WalkDir(output, func(name string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if entry.IsDir() {
						return nil
					}
					rel, err := filepath.Rel(output, name)
					if err != nil {
						return err
					}
					data, err := os.ReadFile(name)
					entries[filepath.ToSlash(rel)] = data
					return err
				}))
			}
			snapshots := decodeBundleSnapshots(t, entries, tc.encoding)
			byName := map[string]*model.SmartBlockSnapshotBase{}
			for _, wrapper := range snapshots {
				if wrapper.SbType == model.SmartBlockType_STRelation {
					s := wrapper.Snapshot.Data
					byName[s.Details.Fields["name"].GetStringValue()] = s
				}
			}
			require.Len(t, byName, 7)
			key := func(name string) string { require.Contains(t, byName, name); return byName[name].Key }
			for _, s := range byName {
				assert.Regexp(t, "^[0-9a-f]{24}$", s.Key)
				assert.Equal(t, s.Key, s.Details.Fields["relationKey"].GetStringValue())
			}
			value := func(id, property string) *types.Value {
				require.Contains(t, snapshots, id)
				return snapshots[id].Snapshot.Data.Details.Fields[key(property)]
			}
			assert.Equal(t, 750.5, value("weekend", "Budget").GetNumberValue())
			date, err := time.Parse(time.RFC3339, "2026-09-18T00:00:00Z")
			require.NoError(t, err)
			assert.Equal(t, float64(date.Unix()), value("weekend", "When").GetNumberValue())
			assert.Equal(t, float64(date.Unix()), value("template-trip", "When").GetNumberValue())
			assert.False(t, byName["When"].Details.Fields["relationFormatIncludeTime"].GetBoolValue())
			assert.Equal(t, 0.0, byName["Expenses"].Details.Fields["relationMaxCount"].GetNumberValue())
			assert.Equal(t, "type-expense", byName["Expenses"].Details.Fields["relationFormatObjectTypes"].GetListValue().Values[0].GetStringValue())
			assert.Equal(t, "hotel", value("weekend", "Expenses").GetListValue().Values[0].GetStringValue())
			assert.Equal(t, "weekend", value("hotel", "Trip").GetListValue().Values[0].GetStringValue())
			booked := value("weekend", "State").GetListValue().Values[0].GetStringValue()
			costBooked := value("hotel", "Cost state").GetListValue().Values[0].GetStringValue()
			assert.NotEqual(t, booked, costBooked)
			assert.Equal(t, "Booked", snapshots[booked].Snapshot.Data.Details.Fields["name"].GetStringValue())
			assert.Equal(t, "blue", snapshots[booked].Snapshot.Data.Details.Fields["relationOptionColor"].GetStringValue())
			assert.Equal(t, booked, value("template-trip", "State").GetListValue().Values[0].GetStringValue())
			optionCount := 0
			for _, wrapper := range snapshots {
				if wrapper.SbType != model.SmartBlockType_STRelationOption {
					continue
				}
				optionCount++
				d := wrapper.Snapshot.Data.Details.Fields
				if d["name"].GetStringValue() == "Dream" {
					assert.Equal(t, "yellow", d["relationOptionColor"].GetStringValue())
					assert.Greater(t, d["orderId"].GetStringValue(), snapshots[booked].Snapshot.Data.Details.Fields["orderId"].GetStringValue())
				}
			}
			assert.Equal(t, 6, optionCount, "unused declared and newly used options are both emitted")
			assert.Equal(t, 20, len(snapshots))
			tripType := snapshots["type-trip"].Snapshot.Data
			assert.Equal(t, []string{"ot-objectType"}, tripType.ObjectTypes)
			assert.Equal(t, "ot-trip", tripType.Details.Fields["uniqueKey"].GetStringValue())
			assert.Equal(t, "template-trip", tripType.Details.Fields["defaultTemplateId"].GetStringValue())
			assert.Equal(t, "rel-"+key("Budget"), tripType.Details.Fields["recommendedFeaturedRelations"].GetListValue().Values[0].GetStringValue())
			assert.Equal(t, "rel-name", tripType.Details.Fields["recommendedRelations"].GetListValue().Values[0].GetStringValue())
			assert.Equal(t, "type-trip", snapshots["template-trip"].Snapshot.Data.Details.Fields["targetObjectType"].GetStringValue())
			weekend := snapshots["weekend"].Snapshot.Data
			assert.Equal(t, []string{"ot-trip"}, weekend.ObjectTypes)
			assert.Equal(t, "🚆", weekend.Details.Fields["iconEmoji"].GetStringValue())
			assert.Equal(t, "sky", weekend.Details.Fields["coverId"].GetStringValue())
			texts := []string{}
			for _, b := range weekend.Blocks {
				if b.GetText() != nil {
					texts = append(texts, b.GetText().Text)
				}
				if b.GetLink() != nil {
					assert.Equal(t, "hotel", b.GetLink().TargetBlockId)
				}
			}
			assert.Equal(t, []string{"Saturday", "Walk across Charles Bridge."}, texts)
			home := snapshots["home"].Snapshot.Data
			setOf := home.Details.Fields["setOf"].GetListValue().Values
			assert.Equal(t, "type-trip", setOf[0].GetStringValue())
			assert.Equal(t, "rel-"+key("Budget"), setOf[1].GetStringValue())
			for _, b := range home.Blocks {
				if b.GetDataview() == nil {
					continue
				}
				view := b.GetDataview().Views[0]
				assert.Equal(t, key("State"), view.Relations[0].Key)
				assert.Equal(t, key("State"), view.Filters[0].RelationKey)
				assert.Equal(t, booked, view.Filters[0].Value.GetListValue().Values[0].GetStringValue())
			}
			profile := &bundleProfile{}
			require.NoError(t, proto.Unmarshal(entries["profile"], profile))
			assert.Equal(t, "Weekends", profile.Name)
			assert.Equal(t, "home", profile.Homepage)
			require.Len(t, profile.Widgets, 3)
			assert.Equal(t, "recentOpen", profile.Widgets[2].Target)
			assert.Equal(t, int32(model.BlockContentWidget_CompactList), profile.Widgets[2].Layout)
			assert.Contains(t, snapshots, "widgets")
		})
	}
}

func TestBundleConversionRejectsBeforeWriting(t *testing.T) {
	for _, test := range []struct{ name, path, data string }{
		{"missing target", "index.json", `{"formatVersion":"2.0","name":"Bad","entrypoint":"missing"}`},
		{"exported document", "objects/export.json", `{"formatVersion":"2.0","kind":"participant","id":"participant-person","properties":{"Name":"Person"}}`},
		{"unknown type", "objects/unknown.json", `{"formatVersion":"2.0","id":"unknown","type":"Typo","properties":{"Name":"Unknown"}}`},
		{"exported dictionary", "properties.json", `{"formatVersion":"2.0","properties":[{"internal_key":"stored-key","name":"Budget","format":"number"}]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			captureCLIWarnings(t)
			fixture := authoringBundleFixture()
			fixture[test.path] = &fstest.MapFile{Data: []byte(test.data)}
			input := writeAuthoringFixture(t, fixture)
			parent := filepath.Join(t.TempDir(), "not-created")
			err := runToV1([]string{"-in", input, "-out", filepath.Join(parent, "bundle"), "-zip"})
			require.Error(t, err)
			_, err = os.Stat(parent)
			assert.ErrorIs(t, err, fs.ErrNotExist)
		})
	}
}

func TestBundleOutputProtection(t *testing.T) {
	input := writeAuthoringFixture(t, authoringBundleFixture())
	output := filepath.Join(t.TempDir(), "existing")
	require.NoError(t, os.WriteFile(output, []byte("keep me"), 0o644))
	for _, extra := range [][]string{nil, {"-zip"}} {
		args := append([]string{"-in", input, "-out", output}, extra...)
		require.ErrorContains(t, runToV1(args), "already exists")
	}
	data, err := os.ReadFile(output)
	require.NoError(t, err)
	assert.Equal(t, "keep me", string(data))
	require.ErrorContains(t, runToV1([]string{"-in", input, "-out", filepath.Join(input, "new", "output")}), "outside")
	alias := filepath.Join(t.TempDir(), "alias")
	require.NoError(t, os.Symlink(input, alias))
	require.ErrorContains(t, runToV1([]string{"-in", input, "-out", filepath.Join(alias, "new", "output")}), "outside")
	require.ErrorContains(t, runToV1([]string{"-in", input, "-out", filepath.Join(t.TempDir(), "bad"), "-encoding", "auto"}), "unknown encoding")
	require.ErrorContains(t, runToV1([]string{"-in", filepath.Join(input, "objects", "trip.json"), "-out", filepath.Join(t.TempDir(), "bad"), "-zip"}), "directory")
}

func TestBundleDoesNotFollowSymlinkDocuments(t *testing.T) {
	input := writeAuthoringFixture(t, authoringBundleFixture())
	require.NoError(t, os.Symlink(filepath.Join(input, "objects", "trip.json"), filepath.Join(input, "linked.json")))
	require.Error(t, runToV1([]string{"-in", input, "-out", filepath.Join(t.TempDir(), "result")}))
}

func TestBundleConvertsExistingAuthoringExample(t *testing.T) {
	captureCLIWarnings(t)
	entries, count, err := convertAuthoringBundle(os.DirFS(filepath.Join("..", "..", "format", "v2", "examples", "habit_tracker")), "pb", "")
	require.NoError(t, err)
	assert.Positive(t, count)
	snapshots := decodeBundleSnapshots(t, entries, "pb")
	assert.Contains(t, snapshots, "type-habit")
	assert.Contains(t, snapshots, "page-start")
}

func TestBundleRejectsOutputCaseCollision(t *testing.T) {
	fixture := authoringBundleFixture()
	fixture["objects/other.json"] = &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","id":"Weekend","type":"Trip","properties":{"Name":"Another weekend"}}`)}
	captureCLIWarnings(t)
	_, _, err := convertAuthoringBundle(fixture, "pb", "")
	require.ErrorContains(t, err, "case-insensitive")
}

func TestBundleWithoutDictionaryOrWidgets(t *testing.T) {
	fixture := fstest.MapFS{
		"index.json": {Data: []byte(`{"formatVersion":"2.0","name":"Notes","entrypoint":"home"}`)},
		"home.json":  {Data: []byte(`{"formatVersion":"2.0","id":"home","type":"Page","properties":{"Name":"Notes"},"blocks":[{"type":"paragraph","text":"Start here."}]}`)},
	}
	entries, count, err := convertAuthoringBundle(fixture, "pb", "")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Len(t, entries, 2)
	profile := &bundleProfile{}
	require.NoError(t, proto.Unmarshal(entries["profile"], profile))
	assert.Equal(t, "home", profile.Homepage)
	assert.Empty(t, profile.Widgets)
}

func TestBundleHyphenatedTypeKeysUseNativeIdentities(t *testing.T) {
	fixture := authoringBundleFixture()
	// Both custom types now use legal v2 keys that Heart cannot parse inside
	// its native ot-<key> identity. Built-in Task and template keys stay intact.
	for _, file := range fixture {
		data := strings.NewReplacer(
			`"type-trip"`, `"type-bookclub-meeting"`,
			`"internal_key":"trip"`, `"internal_key":"bookclub-meeting"`,
			`"type-expense"`, `"type-bookclub-person"`,
			`"internal_key":"expense"`, `"internal_key":"bookclub-person"`,
		).Replace(string(file.Data))
		file.Data = []byte(data)
	}
	fixture["objects/task.json"] = &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","id":"task","type":"Task","properties":{"Name":"Bring a book"}}`)}
	warnings := captureCLIWarnings(t)
	entries, _, err := convertAuthoringBundle(fixture, "pb", "")
	require.NoError(t, err)
	snapshots := decodeBundleSnapshots(t, entries, "pb")
	meeting := snapshots["type-bookclub-meeting"].Snapshot.Data
	person := snapshots["type-bookclub-person"].Snapshot.Data
	for _, typeSnapshot := range []*model.SmartBlockSnapshotBase{meeting, person} {
		assert.Regexp(t, `^[0-9a-f]{24}$`, typeSnapshot.Key)
		uniqueKey := typeSnapshot.Details.Fields["uniqueKey"].GetStringValue()
		assert.Equal(t, "ot-"+typeSnapshot.Key, uniqueKey)
		assert.Len(t, strings.Split(uniqueKey, "-"), 2, "Heart's UnmarshalUniqueKey accepts exactly one separator")
	}
	assert.NotEqual(t, meeting.Key, person.Key)
	assert.Equal(t, []string{"ot-" + meeting.Key}, snapshots["weekend"].Snapshot.Data.ObjectTypes)
	assert.Equal(t, []string{"ot-" + person.Key}, snapshots["hotel"].Snapshot.Data.ObjectTypes)
	assert.Equal(t, []string{"ot-task"}, snapshots["task"].Snapshot.Data.ObjectTypes)
	template := snapshots["template-trip"].Snapshot.Data
	assert.Equal(t, []string{"ot-template", "ot-" + meeting.Key}, template.ObjectTypes)
	assert.Equal(t, "type-bookclub-meeting", template.Details.Fields["targetObjectType"].GetStringValue())
	assert.Equal(t, "type-bookclub-meeting", snapshots["home"].Snapshot.Data.Details.Fields["setOf"].GetListValue().Values[0].GetStringValue())
	for _, wrapper := range snapshots {
		if wrapper.SbType != model.SmartBlockType_STRelation {
			continue
		}
		details := wrapper.Snapshot.Data.Details.Fields
		if details["name"].GetStringValue() == "Expenses" {
			assert.Equal(t, "type-bookclub-person", details["relationFormatObjectTypes"].GetListValue().Values[0].GetStringValue())
		}
	}
	assert.Contains(t, warnings.String(), `internal key "bookclub-meeting"`)
	assert.Contains(t, warnings.String(), `internal key "bookclub-person"`)
	again, _, err := convertAuthoringBundle(fixture, "pb", "")
	require.NoError(t, err)
	repeated := decodeBundleSnapshots(t, again, "pb")
	assert.Equal(t, meeting.Key, repeated["type-bookclub-meeting"].Snapshot.Data.Key, "repeated conversion keeps the native type identity")
}
