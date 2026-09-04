package anyblockjson

// refs.go — object references (§9): the informative `#name` suffix and the
// participant fold.
//
// An object reference in this format is a full id, always (§9a deleted the
// compaction legend). Two amendments make one readable without ceasing to be
// an address:
//
//   - **The `#name` suffix.** A reference MAY carry `#<name>` after the id —
//     `bafyrei…#local_first_ux` — where the name is the referenced object's
//     display name normalized into an identifier grammar (refNameNormalize:
//     letters, digits, `_`, combining marks, nothing else). Key spellings
//     stopped being normalized when raw naming landed; the suffix still is,
//     because its grammar is what keeps the `#` split safe.
//     The suffix is INFORMATIVE ONLY: import trims it at the first `#` and
//     never resolves it, so a stale name costs nothing and two objects
//     sharing one name collide on nothing. It exists so a human or a model
//     reading a document sees what a reference points at instead of a
//     59-character CID. A bare id with no suffix is equally valid, and is
//     what a writer with no name in hand writes.
//
//   - **The participant fold.** `_participant_<spaceId>_<identity>` is a
//     derived id: the space id is the document's own space restated, and the
//     identity is the whole of the content. When Options.SpaceId names the
//     space, export folds the composite down to `participant-<identity>`
//     and import rebuilds the composite (domain.NewParticipantId) — 135
//     characters down to 60, and the same member re-addresses correctly
//     when a document crosses spaces, because the reader rebuilds against
//     ITS space. The prefix is a statement where the bare identity was
//     shape inference, and `-` is outside every ordinary id alphabet, so no
//     ordinary id is or begins one. A bare identity is still read (input
//     compatibility with documents written before the prefix), never
//     written.
//
// The split at `#` is unconditional and safe from both ends, verified rather
// than assumed: no id form this format writes can contain `#` (CIDs are
// base32 `[a-z2-7]`, participant ids base32+base58, `_ot`/`_br` ids are
// `[a-zA-Z0-9_]` across all 223 bundled keys, `_date_…`/`_missing_object`
// are fixed shapes; measured over 37,429 production documents: zero
// id-shaped values contain `#`) — and the name half is normalized through a
// grammar that admits no `#` either.

import (
	"encoding/binary"
	"strings"
	"unicode"

	"github.com/ipfs/go-cid"
	"github.com/mr-tron/base58/base58"
	"golang.org/x/text/unicode/norm"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/filterstring"
	"github.com/anyproto/any-block/format/v1/model"
)

// ObjectNameResolver names an object for the informative reference suffix
// (§9). It is the object-namespace sibling of ParticipantResolver, and it is
// export-only: import trims the suffix without ever asking anyone.
//
// A resolver that cannot name an id returns false and the reference is
// written bare — never with a partial or invented suffix. An empty or
// whitespace name is treated as no name at the seam (refNameLabel), the same
// discipline the participant seam applies, so an implementation answering
// ("", true) cannot put a dangling `#` on every reference in an export.
type ObjectNameResolver interface {
	ObjectName(id string) (string, bool)
}

// ObjectExistenceResolver answers whether the space's store holds an object
// under an id — the question behind the missing-reference rule (§9): a
// reference to an object that does not exist in the SPACE is not written as
// if it did. It is an optional capability of Options.ResolveObjectNames,
// discovered by type assertion (the TypeResolver pattern, §2d): the resolver
// that can NAME an object — one point lookup on the space index — is the one
// that can also say whether the row is there at all, and a caller without it
// keeps a well-defined degradation: nothing is rewritten and nothing is
// dropped, because the absence of an answer is not evidence of absence.
//
// ObjectName is NOT this question and must never stand in for it: its ok is
// `name != ""`, so it answers "no" for an object that exists UNTITLED — and
// untitled objects are common. An export that conflated the two would
// rewrite live references to `_missing_object`.
//
// known=false means the resolver could not ask (a store failure): the caller
// treats the reference exactly as if the capability were absent. exists is
// a statement about the store's rows, tombstones included — a deleted
// object keeps an index row, so a reference to it is NOT missing: the id
// still means something in this space.
type ObjectExistenceResolver interface {
	ObjectExists(id string) (exists, known bool)
}

// ObjectDeletionResolver answers whether an id names an object the space
// DELETED — a tombstone: the index keeps a row stripped to its bookkeeping
// (`{id, spaceId, isDeleted, sync*}`) and nothing else.
//
// It is deliberately separate from ObjectExists, which counts a tombstone as
// existing and says so: "a deleted object keeps an index row, so a reference
// to it is NOT missing: the id still means something in this space". That
// rule stands for every reference slot but one. An ICON is the exception,
// because an icon is OPTIONAL: a link or a mention block must have a target,
// so a dangling one is rewritten to the sentinel rather than dropped, but an
// object with no icon is an ordinary object. Measured over a 77-space
// export, 134 bookmark documents shipped an icon pointing at a favicon whose
// file object had been deleted — every one of the 134 confirmed a tombstone
// in its own space's store.
//
// known=false means the resolver could not ask; the caller then treats the
// reference as live, so a store failure never removes an icon.
type ObjectDeletionResolver interface {
	ObjectDeleted(id string) (deleted, known bool)
}

// DroppedDeletedIconRef reports that an icon image reference names an object
// the space deleted, so export drops the icon and falls through to whatever
// channel is left (§2b) — the same fall-through an image that is not an
// object id already takes.
//
// Exported because snapshotdiff must apply the SAME predicate: `iconImage`
// is a DETAIL, so without this the comparator reads every dropped icon as
// data loss — the drift class that once produced 1,344 false failures in a
// single sweep (§11).
func DroppedDeletedIconRef(opts Options, id string) bool {
	if !isObjectIdShaped(id) {
		return false
	}
	res, ok := opts.ResolveObjectNames.(ObjectDeletionResolver)
	if !ok {
		return false
	}
	deleted, known := res.ObjectDeleted(id)
	return known && deleted
}

// isObjectIdShaped reports whether s parses as a content id (CID) — the
// shape of every object and file id a space actually mints. It is the gate
// that keeps the existence question OFF everything that is not a space
// store row's address: derived ids (`_date_…` is virtual, `_ot…`/`_br…`
// bundled urls and cross-space participant composites resolve against other
// authorities than this space's index), account identities, type and
// property keys, doc-local block ids — none of these parse as a CID, so
// none can be declared missing by a store that was never their authority.
// The cheap length gate mirrors isAccountIdentity's: no CID is shorter than
// 46 characters, and nearly every non-id fails there.
func isObjectIdShaped(s string) bool {
	if len(s) < 46 {
		return false
	}
	_, err := cid.Decode(s)
	return err == nil
}

// missingFromSpace reports that id names an object the wired store says the
// space does not hold — the only fact that may rewrite or drop a reference
// (§9). Three gates, each fail-safe toward "not missing": the id must be
// object-id-shaped (isObjectIdShaped — an id the space index was never the
// authority for cannot be missing from it), the existence capability must be
// wired (a package-only export has no store to ask, and "missing from this
// EXPORT" is not "missing from the space"), and the store must actually
// answer (known) — a store failure leaves the reference untouched.
func missingFromSpace(opts Options, id string) bool {
	if !isObjectIdShaped(id) {
		return false
	}
	res, ok := opts.ResolveObjectNames.(ObjectExistenceResolver)
	if !ok {
		return false
	}
	exists, known := res.ObjectExists(id)
	return known && !exists
}

// DroppedMissingObjectRef reports whether export drops entry from a
// LIST-valued reference slot — an objects/files property value (§3), a
// property document's `object_types` (§2d): the stored `_missing_object`
// sentinel, or an object id the wired store says the space does not hold.
// A list expresses absence by being shorter; singular slots rewrite to the
// sentinel instead (§9) and are not this predicate's business.
//
// Exported because snapshotdiff — the comparator behind the corpus sweep —
// must apply the SAME predicate to both sides, or every dropped-by-design
// entry reports as data loss (the drift class that once produced 1,344
// false failures in one sweep, §11). With no capability wired it drops
// nothing, sentinel included: a package-only export passes every entry
// through verbatim.
func DroppedMissingObjectRef(opts Options, entry string) bool {
	if entry == missingObjectId {
		_, ok := opts.ResolveObjectNames.(ObjectExistenceResolver)
		return ok
	}
	return missingFromSpace(opts, entry)
}

// refNameSep splits an object reference from its informative name suffix.
// The FIRST occurrence splits (§9): the id half can never contain one, and
// the name half never does either once normalized, so first-vs-last is not a
// choice between behaviours — it is the same answer stated defensively.
const refNameSep = "#"

// maxRefNameLen bounds the suffix. The suffix is a glanceable hint, not an
// address, so a name that normalizes past the bound is truncated rather than
// dropped — truncation invents nothing here, unlike a key label (label.go),
// which IS an address and refuses instead.
const maxRefNameLen = 64

// splitRefName splits a reference at the first `#` into the id and the
// informative name. A reference with no `#`, and the degenerate `#…` whose
// id half would be empty, split into themselves and no name: import never
// invents an empty id out of a malformed reference.
func splitRefName(ref string) (id, name string) {
	if i := strings.Index(ref, refNameSep); i > 0 {
		return ref[:i], ref[i+1:]
	}
	return ref, ""
}

// trimRefName is the import half of the suffix: the id, with the informative
// name dropped unread (§9).
func trimRefName(ref string) string {
	id, _ := splitRefName(ref)
	return id
}

// refNameLabel normalizes a display name into the suffix grammar
// (refNameNormalize below), bounded by maxRefNameLen. An empty answer means
// no suffix. The grammar admits no `#`, which is the writer's half of the
// split guarantee: a raw display name here would break the split from both
// ends.
func refNameLabel(name string) string {
	label := refNameNormalize(name)
	if runes := []rune(label); len(runes) > maxRefNameLen {
		label = strings.TrimRight(string(runes[:maxRefNameLen]), "_")
	}
	return label
}

// refNameNormalize turns a display name into the `#name` suffix grammar —
// letters of any script, digits, `_`, combining marks — or "" when nothing
// is left to name.
//
// This is the identifier normalization that used to mint KEY labels
// (label.go), surviving here for its one remaining surface. Key spellings
// are raw names now and need no normalization at all; the ref suffix still
// does, because its grammar is what makes the `#` split safe — a raw
// display name may contain `#`, and the suffix must not. The rules are
// unchanged from the key-label era on purpose: the suffix is informative
// and trimmed unread, so nothing depends on its exact shape, and keeping
// the bytes stable keeps every already-written reference identical on its
// next export.
//
// Three decisions worth keeping stated, because each has a plausible
// alternative:
//
//   - **NFC, lowercase, separators collapse to `_`.** Two visually
//     identical names must not suffix differently between exports.
//   - **Combining marks are kept with their letter.** In Devanagari, Thai,
//     Bengali, Tamil, Khmer and Myanmar the vowels ARE marks; dropping them
//     does not shorten a word, it changes it — मिल/मूल/मल/मैल would all
//     become मल.
//   - **A leading `_` run is content, not a gap** — integrations namespace
//     themselves `__amemory_…` in their names — while interior runs
//     collapse and a trailing run trims; and a result that starts with a
//     digit or is a filter-grammar keyword takes a leading `_`, the escape
//     the suffix inherited from the key grammar and keeps for byte
//     stability.
func refNameNormalize(s string) string {
	if s == "" {
		return ""
	}
	lead := 0
	for _, r := range s {
		if r != '_' {
			break
		}
		lead++
	}
	var b strings.Builder
	gap := false // a separator run is pending, emitted only before the next letter
	for _, r := range norm.NFC.String(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if gap && b.Len() > 0 {
				b.WriteRune('_')
			}
			gap = false
			b.WriteRune(unicode.ToLower(r))
		case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r):
			// a mark cannot start a token, and one arriving with a pending
			// separator is malformed input, not a word
			if b.Len() > 0 && !gap {
				b.WriteRune(r)
			}
		default:
			gap = true // `_` included: runs collapse and edges trim
		}
	}
	label := strings.Repeat("_", lead) + b.String()
	if label == "" || strings.Trim(label, "_") == "" {
		return ""
	}
	if !filterstring.IsBareKey(label) {
		label = "_" + label
	}
	if !filterstring.IsBareKey(label) {
		// unreachable by construction — every rune is already an identPart,
		// so the only faults are a leading digit and a keyword, both cured
		// above. It is a guard rather than a path: IsBareKey is another
		// package's rule and may grow one, and the honest degradation is no
		// suffix at all.
		return ""
	}
	return label
}

// isAccountIdentity reports whether s is a member's account identity — the
// base58 strkey form with its account-address version byte and CRC16-XMODEM
// checksum intact. The checksum is what makes this a CLASSIFIER
// rather than a heuristic: no CID, bson id, or `_`-prefixed derived id can
// decode as one, so an identity in a reference slot is unambiguous.
func isAccountIdentity(s string) bool {
	if len(s) < 40 || len(s) > 64 {
		return false // cheap gate: real identities are 48 characters
	}
	raw, err := base58.FastBase58Decoding(s)
	if err != nil || len(raw) != 35 || raw[0] != 0x5b {
		return false // version byte + 32-byte Ed25519 key + 2-byte checksum
	}
	want := binary.LittleEndian.Uint16(raw[len(raw)-2:])
	return crc16XMODEM(raw[:len(raw)-2]) == want
}

func crc16XMODEM(data []byte) uint16 {
	var crc uint16
	for _, value := range data {
		crc ^= uint16(value) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// foldParticipantRef is the export half of the participant fold (§9):
// `_participant_<SpaceId>_<identity>` becomes `participant-<identity>`. It
// folds ONLY what unfoldParticipantRef provably rebuilds — the space
// embedded in the id must be this run's own SpaceId (a cross-space
// participant ref would otherwise silently re-home on import), the identity
// must classify as one, and the composite must round-trip through
// domain.NewParticipantId byte-identically. With no SpaceId the fold is off
// in both directions: folding on export without the paired import being
// able to rebuild would land a derived id where a composite belongs.
func (o Options) foldParticipantRef(id string) string {
	if o.SpaceId == "" || !strings.HasPrefix(id, domain.ParticipantPrefix) {
		return id
	}
	spaceId, identity, err := domain.ParseParticipantId(id)
	if err != nil || spaceId != o.SpaceId || !isAccountIdentity(identity) {
		return id
	}
	if domain.NewParticipantId(o.SpaceId, identity) != id {
		return id
	}
	return ParticipantRefPrefix + identity
}

// FoldParticipantId is the exported form of the participant fold, for
// callers that must agree with the envelope id Marshal writes WITHOUT
// marshalling: the exporter's path plan names a document file by its
// envelope id (bundle/DESIGN.md §1.3), and a participant document's
// envelope id is its folded `participant-<identity>`. Same gates as the
// internal fold — no spaceId, a foreign space, a non-identity tail, or a
// composite that does not round-trip all decline and return id unchanged —
// which is exactly when Marshal keeps the composite as the envelope id, so
// the plan and the envelope cannot disagree.
func FoldParticipantId(spaceId, id string) string {
	return Options{SpaceId: spaceId}.foldParticipantRef(id)
}

// participantRefIdentity classifies a folded participant reference: the
// canonical `participant-<identity>`, or — input compatibility with
// documents written before the prefix — a bare identity. The identity's
// own strkey checksum is the classifier either way (isAccountIdentity), so
// no CID, bson id or `_`-prefixed derived id can pass as one.
func participantRefIdentity(ref string) (identity string, ok bool) {
	identity = strings.TrimPrefix(ref, ParticipantRefPrefix)
	if !isAccountIdentity(identity) {
		return "", false
	}
	return identity, true
}

// unfoldParticipantRef is the import half: a folded participant reference
// in an object reference slot rebuilds this space's participant id. Gated
// on the exact classifier the fold used, so unfold(fold(x)) == x and
// fold(unfold(y)) == y for every id either side touches.
func (o Options) unfoldParticipantRef(id string) string {
	if o.SpaceId == "" {
		return id
	}
	identity, ok := participantRefIdentity(id)
	if !ok {
		return id
	}
	return domain.NewParticipantId(o.SpaceId, identity)
}

// typeRefKey classifies a type reference by its spelling: the canonical
// `type-<internal_key>`, or — input compatibility, never written — the
// platform's own `ot-<key>` unique-key form that older documents carry. The
// key must pass the fold gate; a `type-` string whose tail does not is not
// a derived id this format would have written and passes through as it
// stands.
func typeRefKey(ref string) (key string, ok bool) {
	switch {
	case strings.HasPrefix(ref, TypeRefPrefix):
		key = ref[len(TypeRefPrefix):]
	case strings.HasPrefix(ref, domain.ObjectTypeKeyToIdPrefix):
		key = ref[len(domain.ObjectTypeKeyToIdPrefix):]
	default:
		return "", false
	}
	if !typeKeyFoldable(key) {
		return "", false
	}
	return key, true
}

// typeRef spells a stored type key as its derived id, `type-<key>`, or
// answers "" when the key fails the fold gate (typeKeyFoldable) — the
// caller then keeps whatever spelling it had, the CID for a document id and
// a reference, the vocabulary's spelling for a key slot. A CID is refused
// outright: a type-key slot may hold an object id that no resolver could
// translate (§2d passes it through verbatim, its own address), and an id
// is not a key however well it fits the charset.
func typeRef(key string) string {
	if !typeKeyFoldable(key) || isObjectIdShaped(key) || strings.HasPrefix(key, "_") {
		// a `_`-prefixed value is a platform address, never a key (§1):
		// the `_missing_object` sentinel passes through a target list
		// verbatim, and `type-_missing_object` would be a derived id of
		// nothing
		return ""
	}
	return TypeRefPrefix + key
}

// typeKeyFoldable is the fold gate on a stored type key (§9): `[A-Za-z0-9_]`,
// 1 to 120 characters. Every population a store actually mints passes —
// bundled camelCase keys, 24-hex bson ids, the bare words of legacy
// accounts — and what it refuses is what could not be a filename stem or
// could not be split back: a `-` (the unique-key separator, so no stored
// type key contains one), a path separator, whitespace, a control
// character, or a length that would push the id past the 128-character
// bound an authored id has. A key that fails keeps its CID everywhere,
// document and references alike, so the two never disagree.
func typeKeyFoldable(key string) bool {
	if len(key) == 0 || len(key) > 120 {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
		default:
			return false
		}
	}
	return true
}

// foldTypeRef is the export half of the type fold (§9): a type object's id
// becomes `type-<internal_key>` wherever it is referenced, and on the type
// document's own envelope. It needs the store's answer to "which key does
// this id name" — the TypeResolver capability of Options.ResolveProperties
// (§2d) — and with no resolver it folds NOTHING, so that a document folded
// by one run never sits beside references a resolver-less run could not
// fold. A key the fold gate refuses keeps the id.
func (o Options) foldTypeRef(id string) string {
	tr, ok := o.ResolveProperties.(TypeResolver)
	if !ok || id == "" {
		return id
	}
	key, ok := tr.TypeKeyById(id)
	if !ok || key == "" {
		return id
	}
	if ref := typeRef(key); ref != "" {
		return ref
	}
	return id
}

// unfoldTypeRef is the import half: `type-<key>` (or the legacy `ot-<key>`)
// rebuilds the type object id the target space serves for that key, through
// the same capability. A key the space does not serve stays as written —
// it is then a bundle-local id, exactly what an authored type document's id
// is, and the import wiring relinks it as it relinks every other bundle
// slug (§2c). No resolver, no unfold.
func (o Options) unfoldTypeRef(ref string) string {
	tr, ok := o.ResolveProperties.(TypeResolver)
	if !ok {
		return ref
	}
	key, ok := typeRefKey(ref)
	if !ok {
		return ref
	}
	if id, ok := tr.TypeIdByKey(key); ok && id != "" {
		return id
	}
	return ref
}

// foldRef is the derived-id fold on one reference slot, with no caption
// (§9): the participant fold and the type fold, which cannot both apply to
// one id. It is what every reference slot that takes no `#name` suffix
// writes through — the envelope id, the icon and cover `file`, a callout's
// icon, a view's `default_template_id`/`default_type_id`, mention and
// object-link targets inside text, the index's own references — so that no
// slot can keep an id the document's own envelope would fold.
func (o Options) foldRef(id string) string {
	if folded := o.foldParticipantRef(id); folded != id {
		return folded
	}
	return o.foldTypeRef(id)
}

// unfoldRef inverts foldRef: the import half for the same suffix-free
// slots. No `#` is trimmed — those slots never carry a caption.
func (o Options) unfoldRef(id string) string {
	if unfolded := o.unfoldParticipantRef(id); unfolded != id {
		return unfolded
	}
	return o.unfoldTypeRef(id)
}

// objectRef renders one object reference for a document slot (§9): the
// derived-id fold first, then the informative `#name` suffix when the
// shape asks for it (Options.RefNames) and a resolver names the target. The
// resolver is asked about the STORED id — the composite participant id, not
// the folded form — because that is the id the space indexes. With no
// resolver, or no name, the reference is written bare — never with a
// partial or invented suffix.
func (e *exporter) objectRef(id string) string {
	out := e.opts.foldRef(id)
	if !e.opts.RefNames || e.opts.ResolveObjectNames == nil || id == "" {
		return out
	}
	if !suffixableRef(id) {
		return out
	}
	name, ok := e.opts.ResolveObjectNames.ObjectName(id)
	if !ok {
		return out
	}
	if label := refNameLabel(name); label != "" {
		return out + refNameSep + label
	}
	return out
}

// suffixableRef reports the ids a name suffix belongs on. A date id and the
// missing-object sentinel already say everything they mean, and a dynamic
// filter placeholder (§6.2) is not an object id at all — a suffix on any of
// them would be decoration on a value some other layer must read verbatim.
//
// An id that already carries a `#` is excluded for a different reason: the
// suffix is only written where it is REVERSIBLE. No id this format writes
// contains one, but a snapshot is untrusted (§11) and may hold anything, and
// `x#y` + `#name` reads back as `x` — a different id from the one exported.
// Worse where the id half is empty: `#name` refuses to split at index 0
// (splitRefName), so import returns it whole and the next export appends
// again, one name per generation without bound. Writing such an id bare
// costs a caption on a reference that could not resolve anyway, and buys
// back §11 guarantee 2.
func suffixableRef(id string) bool {
	return !strings.HasPrefix(id, dateIdPrefix) &&
		id != missingObjectId &&
		!isFilterTemplate(id) &&
		!strings.Contains(id, refNameSep)
}

// singularObjectRef renders a SINGULAR reference slot — a block's
// `object_id` (link, bookmark, file kinds, dataview) — under the
// missing-reference rule (§9): a target the space does not hold is written
// as the `_missing_object` sentinel, because omission cannot express "no
// target" here — only deleting the block could, and that would lose the
// fact that a link existed. A target the store DOES hold, the store cannot
// speak for (missingFromSpace's gates), or that already IS the sentinel
// passes to the ordinary objectRef untouched.
//
// The rewrite warns, naming the id: unlike the sentinel — which says
// nothing beyond "gone" — the id is real information, and the warning is
// its last appearance anywhere. After one round trip the slot is a
// fixpoint: the sentinel is kept as-is, so re-exports are byte-stable.
func (e *exporter) singularObjectRef(path, slot, id string) string {
	if missingFromSpace(e.opts, id) {
		e.warn(path, "%s %q names no object in this space and is written as %q — "+
			"the slot cannot say \"no target\" without deleting the block, and the sentinel "+
			"keeps the fact that a reference existed", slot, id, missingObjectId)
		return e.objectRef(missingObjectId)
	}
	return e.objectRef(id)
}

// droppedMissingListEntry is the LIST half of the missing-reference rule
// (§9): an objects/files property value entry, or an `object_types` entry,
// that the space does not hold is dropped — a list expresses absence by
// being shorter. The predicate is the exported DroppedMissingObjectRef, so
// the comparator applies exactly what export applied.
//
// Only a REAL id warns. A stored `_missing_object` sentinel drops silently:
// it carries nothing — which object it was is already gone — and the corpus
// holds ~990 of them in property values alone, which would triple a warning
// channel that was just cut down to what is worth reading (§12).
func (e *exporter) droppedMissingListEntry(path, id string) bool {
	if !DroppedMissingObjectRef(e.opts, id) {
		return false
	}
	if id != missingObjectId {
		e.warn(path, "%q names no object in this space and is dropped — "+
			"a list expresses absence by being shorter", id)
	}
	return true
}

// exportMarks applies the reference rules to inline markup (§8, §9). A
// `<mention object_id="…">` whose target the space does not hold is
// rewritten to the `_missing_object` sentinel — a mention is a singular
// slot; dropping the mark would lose the fact that a mention existed while
// its text stayed. Then the derived-id fold (foldRef) runs on every mention,
// object-mark and object-link target, exactly as it runs on every other
// reference slot: a text mention of a member spells `participant-<identity>`
// like the `Assignee` value beside it, so a reader joins both to the same
// document. Copy-on-write: the snapshot's own marks are caller-owned state
// and are never mutated, and the common case — nothing to rewrite — returns
// the input slice untouched.
func (e *exporter) exportMarks(path string, marks []*model.BlockContentTextMark) []*model.BlockContentTextMark {
	out := marks
	copied := false
	replace := func(i int, m *model.BlockContentTextMark, param string) {
		if !copied {
			out = append([]*model.BlockContentTextMark(nil), marks...)
			copied = true
		}
		clone := *m
		clone.Param = param
		out[i] = &clone
	}
	for i, m := range marks {
		if m == nil {
			continue
		}
		switch m.Type {
		case model.BlockContentTextMark_Mention:
			if missingFromSpace(e.opts, m.Param) {
				e.warn(path, "mention target %q names no object in this space and is written as %q — "+
					"the mention's own text stays; only its address is gone", m.Param, missingObjectId)
				replace(i, m, missingObjectId)
				continue
			}
			if folded := e.opts.foldRef(m.Param); folded != m.Param {
				replace(i, m, folded)
			}
		case model.BlockContentTextMark_Object:
			if folded := e.opts.foldRef(m.Param); folded != m.Param {
				replace(i, m, folded)
			}
		case model.BlockContentTextMark_Link:
			// a Link whose destination is the object deep link renders as an
			// object mark (§8.3), so its id is a reference slot too
			if id, ok := parseObjectLink(m.Param); ok {
				if folded := e.opts.foldRef(id); folded != id {
					replace(i, m, objectLinkDest(folded))
				}
			}
		}
	}
	return out
}

// unfoldMarks is the import half of exportMarks: every mention, object-mark
// and object-link target rebuilds through unfoldRef. In place — the marks
// were just parsed and are the importer's own.
func (imp *importer) unfoldMarks(marks []*model.BlockContentTextMark) {
	for _, m := range marks {
		if m == nil {
			continue
		}
		switch m.Type {
		case model.BlockContentTextMark_Mention, model.BlockContentTextMark_Object:
			m.Param = imp.opts.unfoldRef(m.Param)
		case model.BlockContentTextMark_Link:
			if id, ok := parseObjectLink(m.Param); ok {
				if unfolded := imp.opts.unfoldRef(id); unfolded != id {
					m.Param = objectLinkDest(unfolded)
				}
			}
		}
	}
}

// ParticipantRefPrefix is the derived-id prefix of a participant (§9): a
// participant document's id, and every reference to it, is
// `participant-<identity>`.
const ParticipantRefPrefix = "participant-"

// TypeRefPrefix is the derived-id prefix of a type (§9): a type document's
// id, and every reference to it, is `type-<internal_key>`.
const TypeRefPrefix = "type-"

// FoldDocumentId is the derived-id fold on a document's own envelope id, for
// callers that must agree with what Marshal writes WITHOUT marshalling —
// the bundle's path plan names a file by its envelope id (bundle/DESIGN.md
// §1.3). It runs the same gates as the reference fold, so the plan and the
// envelope cannot disagree: no SpaceId, no participant fold; no
// TypeResolver, no type fold.
func FoldDocumentId(opts Options, id string) string {
	return opts.foldRef(id)
}

// dateIdPrefix marks a virtual date object id (pkg/lib/localstore/addr).
const dateIdPrefix = "_date_"

// missingObjectId is the dangling-reference sentinel stored details carry
// (pkg/lib/localstore/addr.MissingObject).
const missingObjectId = "_missing_object"

// MissingObjectId is missingObjectId for the round-trip comparator, which
// lives in its own package and must apply the very sentinel export applies —
// the two cannot be allowed to spell it differently.
const MissingObjectId = missingObjectId

// objectRef reads one object reference back (§9): the informative suffix is
// trimmed at the first `#`, unread, and a folded participant id unfolds into this
// space's participant id. Everything else passes verbatim, exactly as
// before the suffix existed — which is what keeps a bare id and a suffixed
// id importing identically.
func (imp *importer) objectRef(ref string) string {
	id := trimRefName(ref)
	// A folded participant reference in a reference slot is the folded
	// half of a participant id (§9), and only a space can rebuild it. A
	// reader that names none would store the folded form where the
	// composite belongs — a reference to an object that does not exist, in
	// silence. The classifier is exact (a strkey checksum), so the reader
	// KNOWS this has happened and says so, once, in build. It may not
	// refuse: Validate never sees Options, so refusing here would put the
	// two surfaces into disagreement over one document (§12 I2).
	if imp.opts.SpaceId == "" {
		if _, folded := participantRefIdentity(id); folded {
			imp.foldedUnrebuilt = true
		}
		return id
	}
	return imp.opts.unfoldRef(id)
}
