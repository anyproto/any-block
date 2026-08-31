package constant

import "slices"

type OptionColor string

func (color OptionColor) String() string {
	return string(color)
}

const (
	ColorGrey   OptionColor = "grey"
	ColorYellow OptionColor = "yellow"
	ColorOrange OptionColor = "orange"
	ColorRed    OptionColor = "red"
	ColorPink   OptionColor = "pink"
	ColorPurple OptionColor = "purple"
	ColorBlue   OptionColor = "blue"
	ColorIce    OptionColor = "ice"
	ColorTeal   OptionColor = "teal"
	ColorLime   OptionColor = "lime"
)

var colors = []OptionColor{
	ColorGrey, ColorYellow, ColorOrange, ColorRed, ColorPink,
	ColorPurple, ColorBlue, ColorIce, ColorTeal, ColorLime,
}

func OptionColors() []OptionColor {
	return slices.Clone(colors)
}
