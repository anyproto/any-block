package bundle

// options.go — the Options a bundle refuses before publishing any documents.
//
// Derived type references must resolve within the bundle.

import (
	"fmt"

	"github.com/anyproto/any-block/codec/anyblockjson"
)

// refuseDocumentOnlyOptions is the bundle seam's admission check on a codec
// Options value. Both constructors run it — BuildPlan and NewComposer take
// Options separately, so a caller can hand the mode to one and not the other,
// and a boundary either door can be walked around is not one.
//
// It runs at CONSTRUCTION, before a path is fixed or a snapshot observed,
// rather than at Finish: a caller told at Finish has already emitted every
// document of the space and can do nothing with the news.
//
// seam is the caller's own error prefix, so the refusal reads like every
// other refusal that call makes.
func refuseDocumentOnlyOptions(opts anyblockjson.Options, seam string) error {
	if !opts.NoDerivedTypeIds {
		return nil
	}
	return fmt.Errorf("%s: Options.NoDerivedTypeIds is a single document's export mode (SPEC §9) "+
		"and a bundle cannot be composed with it — a bundle reaches a type document by its derived id, "+
		"type-<internal_key>, which every typed document spells out in type_internal_key and which has been "+
		"the only road there since the manifest lost its type table (§2c, §15 #26); declining the fold files "+
		"that document under its store id, so the road is gone, properties.json goes on spelling the same type "+
		"type-<key> while no document does, and a template_for naming a type document the bundle does not carry "+
		"stops being reported at all. Pass these Options to anyblockjson.Marshal on ONE document, for a consumer "+
		"that addresses a type by its own controlled key or by the store id its object endpoint resolves, and "+
		"compose bundles with the mode off", seam)
}
