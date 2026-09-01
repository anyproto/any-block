// Package testfixtures provides deterministic, conspicuously synthetic values
// for tests that need credential-, identifier-, or content-address-shaped data.
// It contains no captured user, production, account, invite, or analytics data.
package testfixtures

import (
	"encoding/binary"
	"strings"

	"github.com/mr-tron/base58/base58"
)

const (
	// SpaceID preserves the 73-byte <cid-like>.<suffix> shape used by participant tests.
	SpaceID = "bafyreiaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.synthetic0000"
	// ObjectID, ObjectIDAlt, and ContentID are conspicuously synthetic, valid-shape
	// content addresses for tests where the address namespace itself is semantic.
	ObjectID    = "bafyreiaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	ObjectIDAlt = "bafyreigggggggggggggggggggggggggggggggggggggggggggggggggggg"
	ContentID   = "bafybeiaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	InviteFileKey       = "SYNTHETIC_INVITE_KEY_0000001"
	InviteGuestFileKey  = "SYNTHETIC_GUEST_KEY_000000000"
	RequestMetadataKey  = "SYNTHETIC_REQUEST_KEY_0000000"
	AnalyticsSpaceID    = "SYNTHETIC-ANALYTICS-ID-000001"
	AnalyticsContext    = "SYNTHETIC_ANALYTICS_CONTEXT"
	FileVariantKey      = "SYNTHETIC_FILE_VARIANT_KEY_00000000000000000000000000"
	FileVariantID       = ContentID
	FileVariantChecksum = "SYNTHETIC_CHECKSUM_000000000000000000000000000000000"
	FileVariantOptions  = "SYNTHETIC_VARIANT_OPTIONS_000000000000000000"
	ShortFileVariantKey = "SYNTHETIC_FILE_KEY_000000000000000000"
	AnalyticsObjectID   = "bafyreiaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

var (
	// AccountIdentity and AlternateAccountIdentity are generated from fixed
	// byte sequences. They are valid 48-character account strkeys, but are not
	// copied from or usable by any account.
	AccountIdentity          = accountIdentity(0x10)
	AlternateAccountIdentity = accountIdentity(0x80)
)

// ParticipantID encodes a synthetic account identity using the same spelling
// contract as the production identifier, without importing the codec domain.
func ParticipantID(spaceID, identity string) string {
	return "_participant_" + strings.Replace(spaceID, ".", "_", 1) + "_" + identity
}

func accountIdentity(seed byte) string {
	raw := make([]byte, 35)
	raw[0] = 0x5b // account-address version
	for i := 1; i <= 32; i++ {
		raw[i] = seed + byte(i-1)
	}
	binary.LittleEndian.PutUint16(raw[len(raw)-2:], crc16XMODEM(raw[:len(raw)-2]))
	return base58.FastBase58Encoding(raw)
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
