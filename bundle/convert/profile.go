package convert

import (
	ab "github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/gogo/protobuf/proto"
)

// These are the fields used from Heart's pb.Profile and pb.WidgetBlock
// (pb/protos/snapshot.proto). Keeping the wire-compatible subset here avoids
// pulling the application and its services into this standalone CLI. The root
// profile is always binary protobuf, even when snapshots use JSON encoding.
// Profile is the wire-compatible subset of native archive metadata.
type Profile struct {
	Name     string           `protobuf:"bytes,1,opt,name=name,proto3"`
	Homepage string           `protobuf:"bytes,5,opt,name=spaceDashboardId,proto3"`
	Widgets  []*ProfileWidget `protobuf:"bytes,9,rep,name=widgets,proto3"`
}

func (p *Profile) Reset()         { *p = Profile{} }
func (p *Profile) String() string { return proto.CompactTextString(p) }
func (*Profile) ProtoMessage()    {}

// ProfileWidget identifies one sidebar target in the native archive.
type ProfileWidget struct {
	Layout int32  `protobuf:"varint,1,opt,name=layout,proto3"`
	Target string `protobuf:"bytes,2,opt,name=targetObjectId,proto3"`
	Limit  int32  `protobuf:"varint,3,opt,name=objectLimit,proto3"`
}

func (p *ProfileWidget) Reset()         { *p = ProfileWidget{} }
func (p *ProfileWidget) String() string { return proto.CompactTextString(p) }
func (*ProfileWidget) ProtoMessage()    {}

func encodeBundleProfile(idx *ab.Index) ([]byte, error) {
	profile := &Profile{Name: idx.Name, Homepage: ab.WireHomepage(idx.SpaceHomepage())}
	// Read enum values from the same builder that emits the sidebar snapshot,
	// so adding a supported layout cannot drift between the two representations.
	widgets, err := ab.WidgetsSnapshot(idx)
	if err != nil {
		return nil, err
	}
	if widgets != nil {
		for _, block := range widgets.Blocks {
			widget, ok := block.Content.(*model.BlockContentOfWidget)
			if !ok {
				continue
			}
			w := idx.Widgets[len(profile.Widgets)]
			profile.Widgets = append(profile.Widgets, &ProfileWidget{Layout: int32(widget.Widget.Layout), Target: ab.WireWidgetTarget(w.Target), Limit: w.Limit})
		}
	}
	return proto.Marshal(profile)
}
