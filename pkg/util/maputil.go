package util

// GetString safely extracts a string value from a map.
// Returns empty string if key doesn't exist or value is not a string.
func GetString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt safely extracts an int value from a map.
// Returns 0 if key doesn't exist or value is not a number.
func GetInt(m map[string]interface{}, key string) int {
	if m == nil {
		return 0
	}
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}
