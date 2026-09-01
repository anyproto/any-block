package pbtypes

import "github.com/gogo/protobuf/types"

func Int64(value int64) *types.Value {
	return &types.Value{Kind: &types.Value_NumberValue{NumberValue: float64(value)}}
}

func String(value string) *types.Value {
	return &types.Value{Kind: &types.Value_StringValue{StringValue: value}}
}

func StringList(values []string) *types.Value {
	items := make([]*types.Value, 0, len(values))
	for _, value := range values {
		items = append(items, String(value))
	}
	return &types.Value{Kind: &types.Value_ListValue{ListValue: &types.ListValue{Values: items}}}
}

func Bool(value bool) *types.Value {
	return &types.Value{Kind: &types.Value_BoolValue{BoolValue: value}}
}

func GetStringListValue(value *types.Value) []string {
	if value == nil {
		return nil
	}
	if list, ok := value.Kind.(*types.Value_ListValue); ok {
		result := make([]string, 0, len(list.ListValue.GetValues()))
		for _, item := range list.ListValue.GetValues() {
			if text, ok := item.Kind.(*types.Value_StringValue); ok {
				result = append(result, text.StringValue)
			}
		}
		return result
	}
	if text, ok := value.Kind.(*types.Value_StringValue); ok && text.StringValue != "" {
		return []string{text.StringValue}
	}
	return nil
}

func ValueToInterface(value *types.Value) any {
	if value == nil {
		return nil
	}
	switch kind := value.Kind.(type) {
	case *types.Value_NullValue:
		return nil
	case *types.Value_NumberValue:
		return kind.NumberValue
	case *types.Value_StringValue:
		return kind.StringValue
	case *types.Value_BoolValue:
		return kind.BoolValue
	case *types.Value_StructValue:
		result := make(map[string]any, len(kind.StructValue.GetFields()))
		for key, item := range kind.StructValue.GetFields() {
			result[key] = ValueToInterface(item)
		}
		return result
	case *types.Value_ListValue:
		result := make([]any, len(kind.ListValue.GetValues()))
		for i, item := range kind.ListValue.GetValues() {
			result[i] = ValueToInterface(item)
		}
		return result
	default:
		panic("protostruct: unknown kind")
	}
}
