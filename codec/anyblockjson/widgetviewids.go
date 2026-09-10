package anyblockjson

import (
	"fmt"

	"github.com/anyproto/any-block/format/v1/model"
)

// WidgetViewIDs returns the primary dataview's stored-to-exported view IDs.
// It uses the target object's own label plan, including collision fallback.
// Call with the same snapshot and options used by Marshal. Index widgets imply
// the fixed primary block "dataview"; they do not address arbitrary blocks.
func WidgetViewIDs(sbType model.SmartBlockType, snapshot *model.SmartBlockSnapshotBase, opts Options) map[string]string {
	if snapshot == nil {
		return nil
	}
	var primary *model.Block
	for _, b := range snapshot.Blocks {
		if b != nil && b.Id == dataviewBlockId {
			primary = b
		}
	}
	if primary == nil || len(primary.GetDataview().GetViews()) == 0 {
		return nil
	}
	// Preserve warning-enabled behavior without duplicating diagnostics.
	if opts.OnWarning != nil {
		opts.OnWarning = func(Issue) {}
	}
	e := &exporter{opts: opts, snapshot: snapshot, sbType: sbType, blocks: map[string]*model.Block{}, visited: map[string]bool{}}
	e.indexBlocks()
	if !e.canonicalEmissionPlan().emitted[dataviewBlockId] {
		return nil
	}
	if opts.compactBlockLabels() && !opts.OmitIds {
		e.buildLabelPlan()
	}
	labels := map[string]string{}
	for _, view := range primary.GetDataview().GetViews() {
		if view != nil && view.Id != "" {
			labels[view.Id] = e.localId(view.Id)
		}
	}
	return labels
}

// A kept widget document is rendered before the composer can rewrite index
// selectors. Require its target's mapping instead of guessing a suffix.
func (e *exporter) widgetViewID(block *model.Block, viewID string) (string, error) {
	if !e.opts.compactBlockLabels() || e.opts.OmitIds || !isMintedLocalId(viewID) {
		return viewID, nil
	}
	var target string
	for _, child := range block.ChildrenIds {
		if link := e.blocks[child].GetLink(); link != nil {
			target = link.TargetBlockId
			break
		}
	}
	if e.opts.ResolveWidgetViewID != nil {
		if label, ok := e.opts.ResolveWidgetViewID(target, viewID); ok && label != "" {
			return label, nil
		}
	}
	return "", fmt.Errorf("widget block %q: shortening view %q in target %q requires ResolveWidgetViewID; use the target object's WidgetViewIDs mapping or export this widget document with full IDs", block.Id, viewID, target)
}
