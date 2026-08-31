package anyblockjson

// propertydefinitionrender.go holds the checked writer for the members shared
// by every propertyDefinition home. The type-property exporter uses it now;
// keeping the checks out of typeproperties.go also leaves one seam for the
// dictionary writer to adopt without duplicating the format and option rules.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"unicode/utf8"

	"github.com/anyproto/any-block/codec/anyblockjson/internal/constant"
)

// renderPropertyDefinitionMembers appends all non-identity members of def in
// canonical order. objectTypes are already translated into the vocabulary of
// their enclosing document. A type declaration may not carry the internal
// map format, and its option and target lists must agree with the format; the
// schema and semantic validator enforce the same rules on read.
func renderPropertyDefinitionMembers(m *omap, def PropertyDefinition, objectTypes []string, authorable bool) error {
	format := formatName(def.Format)
	if format == "" {
		return fmt.Errorf("property %q: format %d has no name in this format: the definition cannot state what the property holds", def.Key, def.Format)
	}
	if authorable && format == "map" {
		return fmt.Errorf("property %q: format %q is not authorable in a type property definition", def.Key, format)
	}

	options, err := checkedPropertyOptions(def, format, authorable)
	if err != nil {
		return err
	}
	if len(objectTypes) > 0 && format != "objects" && format != "files" {
		return fmt.Errorf("property %q: object_types is only meaningful on objects/files, not %q", def.Key, format)
	}
	for i, key := range objectTypes {
		if key == "" {
			return fmt.Errorf("property %q: object_types[%d] is empty and cannot name a type", def.Key, i)
		}
	}
	if def.MaxCount < 0 || def.MaxCount > math.MaxInt32 {
		return fmt.Errorf("property %q: max_count %d is outside the range a definition can state (0..%d)", def.Key, def.MaxCount, math.MaxInt32)
	}

	m.setNonEmpty("name", def.Name)
	m.set("format", format)
	m.setNonEmpty("options", options)
	m.setNonEmpty("object_types", stringsToAny(objectTypes))
	m.setNonEmpty("description", def.Description)
	if def.IncludeTime != nil {
		// A pointer false is a declaration, not an absence.
		m.set("include_time", *def.IncludeTime)
	} else if def.IncludeTimeSet {
		// A present nil is explicit JSON null. The presence bit is what keeps
		// that declaration distinct from an omitted member.
		m.set("include_time", nil)
	}
	m.setNonEmpty("max_count", def.MaxCount)
	m.setNonEmpty("readonly", def.Readonly)
	if def.DefaultValue != nil || def.DefaultValueSet {
		value, err := canonicalPropertyDefault(def.DefaultValue)
		if err != nil {
			return fmt.Errorf("property %q: default_value is not JSON: %w", def.Key, err)
		}
		m.set("default_value", value)
	}
	return nil
}

func checkedPropertyOptions(def PropertyDefinition, format string, authorable bool) ([]any, error) {
	if len(def.Options) == 0 {
		return nil, nil
	}
	if format != "select" && format != "multi_select" {
		return nil, fmt.Errorf("property %q: options is only meaningful on select/multi_select, not %q", def.Key, format)
	}
	colors := make(map[string]bool, len(constant.OptionColors()))
	for _, color := range constant.OptionColors() {
		colors[color.String()] = true
	}

	out := make([]any, 0, len(def.Options))
	seen := make(map[string]int, len(def.Options))
	for i, option := range def.Options {
		if option.Name == "" {
			return nil, fmt.Errorf("property %q: options[%d].name is empty", def.Key, i)
		}
		if !utf8.ValidString(option.Name) {
			return nil, fmt.Errorf("property %q: options[%d].name is not valid UTF-8", def.Key, i)
		}
		// Type authoring resolves options by their names and therefore cannot
		// state the same one twice. A dictionary backs up stored options with
		// explicit internal keys; real spaces can contain same-named twins, so
		// that home preserves both and their order.
		if first, duplicate := seen[option.Name]; duplicate && authorable {
			return nil, fmt.Errorf("property %q: options[%d] duplicates option %q at index %d", def.Key, i, option.Name, first)
		}
		seen[option.Name] = i
		if option.Color != "" && !colors[option.Color] {
			return nil, fmt.Errorf("property %q: options[%d].color %q is not an Anytype option color", def.Key, i, option.Color)
		}
		if option.InternalKey != "" {
			if !utf8.ValidString(option.InternalKey) {
				return nil, fmt.Errorf("property %q: options[%d].internal_key is not valid UTF-8", def.Key, i)
			}
		}

		if option.Color == "" && option.InternalKey == "" {
			out = append(out, option.Name)
			continue
		}
		entry := &omap{}
		entry.set("name", option.Name)
		entry.setNonEmpty("color", option.Color)
		entry.setNonEmpty("internal_key", option.InternalKey)
		out = append(out, entry)
	}
	return out, nil
}

// canonicalPropertyDefault verifies that a caller supplied a JSON value and
// converts objects to omap recursively, preserving integer spellings and the
// package's deterministic, non-HTML-escaped canonical writer. Every number
// must fit in a finite float64 because semantic validation and the stored
// proto value model impose that bound. Integers beyond float64's exact range
// remain json.Number: the definition importer preserves them the same way,
// so a successful writer never rounds them on its own read path.
func canonicalPropertyDefault(value any) (any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return orderedJSONValue(decoded, "")
}

func orderedJSONValue(value any, path string) (any, error) {
	switch value := value.(type) {
	case map[string]any:
		m := &omap{}
		for _, key := range sortedStringKeys(value) {
			ordered, err := orderedJSONValue(value[key], path+"/"+escapeJSONPointer(key))
			if err != nil {
				return nil, err
			}
			m.set(key, ordered)
		}
		return m, nil
	case []any:
		out := make([]any, len(value))
		for i := range value {
			ordered, err := orderedJSONValue(value[i], path+"/"+strconv.Itoa(i))
			if err != nil {
				return nil, err
			}
			out[i] = ordered
		}
		return out, nil
	case json.Number:
		if f, err := value.Float64(); err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			if path == "" {
				path = "/"
			}
			return nil, fmt.Errorf("number at %s (%s) is outside the finite float64 range imported values can store", path, value)
		}
		return value, nil
	default:
		return value, nil
	}
}

// normalizeImportedPropertyDefault keeps the established float64 shape for
// ordinary JSON numbers while retaining json.Number when a float64 would
// change the number's mathematical value. This is the import half of
// canonicalPropertyDefault's policy: 100 and 0.1 remain conventional Go JSON
// values, while 9007199254740993 is not rounded to 9007199254740992.
func normalizeImportedPropertyDefault(value any) any {
	switch value := value.(type) {
	case map[string]any:
		for key, nested := range value {
			value[key] = normalizeImportedPropertyDefault(nested)
		}
		return value
	case []any:
		for i, nested := range value {
			value[i] = normalizeImportedPropertyDefault(nested)
		}
		return value
	case json.Number:
		if f, exact := exactPropertyDefaultFloat(value); exact {
			return f
		}
		return value
	default:
		return value
	}
}

func exactPropertyDefaultFloat(number json.Number) (float64, bool) {
	f, err := number.Float64()
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, false
	}
	original, ok := new(big.Rat).SetString(string(number))
	if !ok {
		return 0, false
	}
	rendered, ok := new(big.Rat).SetString(strconv.FormatFloat(f, 'g', -1, 64))
	return f, ok && original.Cmp(rendered) == 0
}
