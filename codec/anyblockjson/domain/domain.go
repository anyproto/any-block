// Package domain contains the small set of identifiers whose encoding is
// part of AnyBlock v2. It intentionally depends on nothing but the standard
// library, so the encoding rules travel without an application behind them.
package domain

import (
	"fmt"
	"strings"
)

const (
	RelationKeyToIdPrefix      = "rel-"
	ObjectTypeKeyToIdPrefix    = "ot-"
	BundledRelationURLPrefix   = "_br"
	BundledObjectTypeURLPrefix = "_ot"
	ParticipantPrefix          = "_participant_"
)

type RelationKey string

func (key RelationKey) String() string {
	return string(key)
}

func (key RelationKey) URL() string {
	return RelationKeyToIdPrefix + string(key)
}

func (key RelationKey) BundledURL() string {
	return BundledRelationURLPrefix + string(key)
}

type TypeKey string

func (key TypeKey) String() string {
	return string(key)
}

func (key TypeKey) URL() string {
	return ObjectTypeKeyToIdPrefix + string(key)
}

func (key TypeKey) BundledURL() string {
	return BundledObjectTypeURLPrefix + string(key)
}

func NewParticipantId(spaceID, identity string) string {
	spaceID = strings.Replace(spaceID, ".", "_", 1)
	return fmt.Sprintf("%s%s_%s", ParticipantPrefix, spaceID, identity)
}

// ValidateSpaceId checks the structural contract required by participant ids.
// A space id is encoded inside a participant id by replacing its one separator
// dot with an underscore, so both sides of that separator must be non-empty and
// the resulting participant id must parse back to the exact original space.
//
// This intentionally validates only the shared participant-id representation,
// not whether the space exists or is accessible to the caller.
func ValidateSpaceId(spaceID string) error {
	if strings.Count(spaceID, ".") != 1 {
		return fmt.Errorf("space id must contain exactly one separator dot")
	}
	parts := strings.SplitN(spaceID, ".", 2)
	if parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("space id components must not be empty")
	}

	const probeIdentity = "space-id-validation-probe"
	participantID := NewParticipantId(spaceID, probeIdentity)
	parsedSpaceID, parsedIdentity, err := ParseParticipantId(participantID)
	if err != nil {
		return fmt.Errorf("space id cannot be encoded in a participant id: %w", err)
	}
	if parsedSpaceID != spaceID || parsedIdentity != probeIdentity {
		return fmt.Errorf("space id does not round-trip through a participant id")
	}
	return nil
}

func ParseParticipantId(participantID string) (spaceID string, identity string, err error) {
	if !strings.HasPrefix(participantID, ParticipantPrefix) {
		return "", "", fmt.Errorf("participant id must start with %s", ParticipantPrefix)
	}
	parts := strings.Split(participantID, "_")
	if len(parts) != 5 {
		return "", "", fmt.Errorf("can't extract space id")
	}
	return parts[2] + "." + parts[3], parts[4], nil
}
