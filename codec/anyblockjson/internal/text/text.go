package text

import "unicode/utf16"

func UTF16RuneCountString(value string) int {
	count := 0
	for _, r := range value {
		count += utf16.RuneLen(r)
	}
	return count
}

func StrToUTF16(value string) []uint16 {
	return utf16.Encode([]rune(value))
}

func UTF16ToStr(value []uint16) string {
	return string(utf16.Decode(value))
}
