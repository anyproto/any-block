package anyblockjson

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/anyproto/any-block/format/v1/model"
)

const memberLayoutWidthPercent = "layout_width_percent"

// IsLayoutWidthWarning reports whether the issue concerns an out-of-range
// layout_width_percent. Both validation and import can report these issues.
func (c IssueCode) IsLayoutWidthWarning() bool {
	return c == IssueCodeLayoutWidthClamped || c == IssueCodeLayoutWidthFraction
}

func supportsLayoutWidth(kind model.SmartBlockType) bool {
	return kind == model.SmartBlockType_Page || kind == model.SmartBlockType_Template
}

// compareWidthPercent compares a JSON decimal with a nonnegative integer
// without rounding at the boundary or expanding an arbitrarily large exponent.
func compareWidthPercent(n json.Number, boundary int) int {
	digits := n.String()
	var exponent int64
	if i := strings.IndexAny(digits, "eE"); i >= 0 {
		// ParseInt saturates at its limits, already far beyond a JSON
		// document's possible number of significant digits.
		exponent, _ = strconv.ParseInt(digits[i+1:], 10, 64)
		digits = digits[:i]
	}
	digits, negative := strings.CutPrefix(digits, "-")
	fractionPlaces := 0
	if i := strings.IndexByte(digits, '.'); i >= 0 {
		fractionPlaces = len(digits) - i - 1
		digits = digits[:i] + digits[i+1:]
	}
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return cmp.Compare(0, boundary)
	}
	if negative {
		return -1
	}
	if boundary == 0 {
		return 1
	}
	target := strconv.Itoa(boundary)
	// Align the decimal points before comparing the significant digits.
	if order := cmp.Compare(exponent, int64(fractionPlaces-len(digits)+len(target))); order != 0 {
		return order
	}
	return strings.Compare(strings.TrimRight(digits, "0"), strings.TrimRight(target, "0"))
}

func clampLayoutWidthPercent(n json.Number, warn func(Issue)) json.Number {
	var endpoint json.Number
	switch {
	case compareWidthPercent(n, 100) < 0:
		endpoint = "100"
	case compareWidthPercent(n, 200) > 0:
		endpoint = "200"
	default:
		return n
	}
	if warn != nil {
		path := "/" + memberLayoutWidthPercent
		warn(Issue{Path: path, Code: IssueCodeLayoutWidthClamped, Message: fmt.Sprintf(
			"%s is outside the 100–200%% range and was clamped to %s%%", n, endpoint)})
		if compareWidthPercent(n, 0) > 0 && compareWidthPercent(n, 1) < 0 {
			warn(Issue{Path: path, Code: IssueCodeLayoutWidthFraction,
				Message: "use a percentage for page width (for example, write 175 for the legacy root width 0.75)"})
		}
	}
	return endpoint
}

// DecodeLayoutWidthPercent converts a single layout_width_percent JSON value
// to the native root block width. Document editors can use the same units,
// range correction and diagnostics as whole-document import.
func DecodeLayoutWidthPercent(raw json.RawMessage, warn func(Issue)) (float64, error) {
	var value any
	err := jsonUnmarshalUseNumber(raw, &value)
	number, ok := value.(json.Number)
	if err != nil || !ok {
		return 0, &ValidationError{Issues: []Issue{{Path: "/" + memberLayoutWidthPercent,
			Message: "layout_width_percent must be a JSON number (100–200%)"}}}
	}
	number = clampLayoutWidthPercent(number, warn)
	percent, fault := jsonNumberRepresentabilityFault(number)
	if fault != "" {
		return 0, numberValidationError("/"+memberLayoutWidthPercent, number.String(), fault)
	}
	return storedLayoutWidth(percent), nil
}

// storedLayoutWidth scales the authored decimal before rounding to the
// snapshot's double. For example, 100.01% stores exactly the same double
// as the legacy JSON value 0.0001.
func storedLayoutWidth(percent float64) float64 {
	// Validation admits exactly decimals equal to this shortest spelling.
	n, _ := new(big.Rat).SetString(strconv.FormatFloat(percent, 'g', -1, 64))
	hundred := big.NewRat(100, 1)
	f, _ := n.Quo(n.Sub(n, hundred), hundred).Float64()
	return f
}

// exportLayoutWidth lifts a legacy double only if the percentage survives
// the v2 numeric transport policy and imports to the exact stored width.
func exportLayoutWidth(width float64) (float64, bool) {
	if math.Signbit(width) || width > 1 || math.IsNaN(width) {
		return 0, false
	}
	p := math.FMA(width, 100, 100)
	// Scaling and rounding the shortest JSON decimal can select a different
	// double from scaling its binary value. Try the adjacent percentages too.
	for _, candidate := range [...]float64{p, math.Nextafter(p, 100), math.Nextafter(p, 200)} {
		if storedLayoutWidth(candidate) == width {
			return candidate, true
		}
	}
	return 0, false
}
