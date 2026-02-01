package cmd

import (
	"fmt"
	"sort"
	"strings"
)

func parseKeyValuePairs(items []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key, value, err := splitKeyValue(trimmed)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}

func parseKeyValueCSV(input string) (map[string]string, error) {
	if strings.TrimSpace(input) == "" {
		return map[string]string{}, nil
	}
	parts := strings.Split(input, ",")
	return parseKeyValuePairs(parts)
}

func formatKeyValuePairs(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return strings.Join(pairs, ", ")
}

func splitKeyValue(value string) (string, string, error) {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid format %q (expected KEY=VALUE)", value)
	}
	key := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])
	if key == "" {
		return "", "", fmt.Errorf("invalid format %q (empty key)", value)
	}
	return key, val, nil
}
