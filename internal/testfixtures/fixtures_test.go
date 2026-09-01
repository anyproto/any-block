package testfixtures

import (
	"encoding/binary"
	"testing"

	"github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyntheticFixtureShapes(t *testing.T) {
	assert.Len(t, SpaceID, 73)
	assert.Len(t, ObjectID, 59)
	assert.Len(t, ObjectIDAlt, 59)
	assert.Len(t, ContentID, 59)
	assert.NotEqual(t, ObjectID, ObjectIDAlt)
	assert.Len(t, AccountIdentity, 48)
	assert.Len(t, AlternateAccountIdentity, 48)
	assert.NotEqual(t, AccountIdentity, AlternateAccountIdentity)
	assert.Len(t, ParticipantID(SpaceID, AccountIdentity), 135)

	for _, identity := range []string{AccountIdentity, AlternateAccountIdentity} {
		raw, err := base58.FastBase58Decoding(identity)
		require.NoError(t, err)
		require.Len(t, raw, 35)
		assert.Equal(t, byte(0x5b), raw[0])
		assert.Equal(t, crc16XMODEM(raw[:len(raw)-2]), binary.LittleEndian.Uint16(raw[len(raw)-2:]))
	}

	for name, fixture := range map[string]struct {
		value string
		want  int
	}{
		"invite":           {InviteFileKey, 28},
		"guest invite":     {InviteGuestFileKey, 29},
		"request metadata": {RequestMetadataKey, 29},
		"analytics space":  {AnalyticsSpaceID, 29},
		"variant key":      {FileVariantKey, 53},
		"variant id":       {FileVariantID, 59},
		"checksum":         {FileVariantChecksum, 52},
		"variant options":  {FileVariantOptions, 44},
		"short key":        {ShortFileVariantKey, 37},
		"analytics object": {AnalyticsObjectID, 59},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Len(t, fixture.value, fixture.want)
		})
	}
}
