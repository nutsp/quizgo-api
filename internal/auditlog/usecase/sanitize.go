package usecase

import (
	"encoding/json"
	"reflect"
	"strings"
)

var sensitiveFields = map[string]struct{}{
	"password":      {},
	"password_hash": {},
	"access_token":  {},
	"refresh_token": {},
	"token":         {},
	"secret":        {},
}

func SanitizeAuditData(data any) any {
	if data == nil {
		return nil
	}
	switch v := data.(type) {
	case map[string]any:
		return sanitizeMap(v)
	default:
		b, err := json.Marshal(data)
		if err != nil {
			return data
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return data
		}
		return sanitizeMap(m)
	}
}

func sanitizeMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if _, sensitive := sensitiveFields[strings.ToLower(k)]; sensitive {
			continue
		}
		out[k] = sanitizeValue(v)
	}
	return out
}

func sanitizeValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return sanitizeMap(val)
	case []any:
		items := make([]any, len(val))
		for i, item := range val {
			items[i] = sanitizeValue(item)
		}
		return items
	default:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Map {
			iter := rv.MapRange()
			m := make(map[string]any)
			for iter.Next() {
				key := iter.Key().String()
				if _, sensitive := sensitiveFields[strings.ToLower(key)]; sensitive {
					continue
				}
				m[key] = sanitizeValue(iter.Value().Interface())
			}
			return m
		}
		return v
	}
}
