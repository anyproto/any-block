package convert

import (
	"encoding/hex"
	ab "github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBundleProfileWireCompatibility(t *testing.T) {
	// Independent wire fixture: Profile.name=1, dashboard=5, widgets=9;
	// WidgetBlock.layout=1 (Link=0 omitted), target=2, limit=3.
	wire, err := hex.DecodeString("0a01582a04686f6d654a081204686f6d651803")
	require.NoError(t, err)
	data, err := encodeBundleProfile(&ab.Index{Name: "X", Entrypoint: "home", Widgets: []ab.Widget{{Target: "home", Limit: 3}}})
	require.NoError(t, err)
	assert.Equal(t, wire, data)
}
