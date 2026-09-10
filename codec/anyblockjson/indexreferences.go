package anyblockjson

import (
	"fmt"

	"github.com/anyproto/any-block/format/v1/model"
)

// IndexReferencesFromSnapshot retains provenance for metadata lifted into the
// index. Call only for a snapshot accepted by the corresponding omission rule.
func IndexReferencesFromSnapshot(sbType model.SmartBlockType, base *model.SmartBlockSnapshotBase, opts Options) []ObjectReference {
	if base == nil {
		return nil
	}
	id := base.GetDetails().GetFields()["id"].GetStringValue()
	var idx Index
	switch sbType {
	case model.SmartBlockType_Workspace:
		IndexFromSpaceSettings(&idx, base)
		refs := idx.ObjectReferences(opts)
		for n := range refs {
			refs[n].ObjectID = id
			switch refs[n].Path {
			case "/homepage":
				refs[n].SourcePath = "/properties/homepage"
			case "/icon/file":
				refs[n].SourcePath = "/properties/iconImage"
			}
		}
		return refs
	case model.SmartBlockType_Widget:
		widgets, links, ok := widgetObjectWidgetsWithSources(base)
		if !ok {
			return nil
		}
		idx.Widgets = widgets
		refs := idx.ObjectReferences(opts)
		for n := range refs {
			refs[n].ObjectID = id
			for j, link := range links {
				if refs[n].Path == fmt.Sprintf("/widgets/%d/target", j) {
					refs[n].SourcePath = "/blocks/" + escapeSourcePathKey(link) + "/object_id"
					break
				}
			}
		}
		return refs
	}
	return nil
}
