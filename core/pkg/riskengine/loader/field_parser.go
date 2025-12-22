package loader

import (
	"fmt"
	"strings"
)

// FieldPath represents a parsed field path
type FieldPath struct {
	Segments []PathSegment
}

// PathSegment represents a segment in a field path
type PathSegment struct {
	Name      string
	IsArray   bool
	ArrayIndex *int // nil means all items ([])
	Field     string // For array items, the field to access
}

// ParseFieldPath parses a field path like "roleRef.name" or "subjects[].kind"
func ParseFieldPath(path string) (*FieldPath, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}

	var segments []PathSegment
	parts := strings.Split(path, ".")

	for _, part := range parts {
		// Check for array notation: field[] or field[index]
		if strings.Contains(part, "[") {
			// Extract field name and array part
			idx := strings.Index(part, "[")
			fieldName := part[:idx]
			arrayPart := part[idx+1 : len(part)-1] // Remove [ and ]

			segment := PathSegment{
				Name:    fieldName,
				IsArray: true,
			}

			if arrayPart == "" {
				// [] means all items
				segment.ArrayIndex = nil
			} else {
				// Try to parse as integer index
				var idx int
				if _, err := fmt.Sscanf(arrayPart, "%d", &idx); err == nil {
					segment.ArrayIndex = &idx
				} else {
					// Not a number, might be a field access like [].kind
					// This will be handled in the next segment
					segment.ArrayIndex = nil
					// The field name after [] is the next part
					if len(parts) > 0 {
						// This is handled by checking if next segment exists
					}
				}
			}

			segments = append(segments, segment)
		} else {
			// Regular field access
			segments = append(segments, PathSegment{
				Name:    part,
				IsArray: false,
			})
		}
	}

	return &FieldPath{Segments: segments}, nil
}

// GetValue extracts value from data using parsed path
func (fp *FieldPath) GetValue(data map[string]interface{}) interface{} {
	current := interface{}(data)

	for i, segment := range fp.Segments {
		if segment.IsArray {
			// Handle array access
			arr, ok := current.([]interface{})
			if !ok {
				// Try to get from map first
				if m, ok := current.(map[string]interface{}); ok {
					val, exists := m[segment.Name]
					if !exists {
						return nil
					}
					arr, ok = val.([]interface{})
					if !ok {
						return nil
					}
				} else {
					return nil
				}
			}

			if segment.ArrayIndex == nil {
				// [] means all items - return array
				// If next segment exists, extract field from each item
				if i+1 < len(fp.Segments) {
					nextSegment := fp.Segments[i+1]
					var results []interface{}
					for _, item := range arr {
						if itemMap, ok := item.(map[string]interface{}); ok {
							if val, exists := itemMap[nextSegment.Name]; exists {
								results = append(results, val)
							}
						}
					}
					return results
				}
				return arr
			} else {
				// Specific index
				idx := *segment.ArrayIndex
				if idx < 0 || idx >= len(arr) {
					return nil
				}
				current = arr[idx]
			}
		} else {
			// Regular field access
			if m, ok := current.(map[string]interface{}); ok {
				val, exists := m[segment.Name]
				if !exists {
					return nil
				}
				current = val
			} else {
				return nil
			}
		}
	}

	return current
}

// SimpleFieldAccess is a lightweight field accessor (no caching for now)
func SimpleFieldAccess(data map[string]interface{}, field string) interface{} {
	// Try simple dot notation first
	if !strings.Contains(field, "[") {
		parts := strings.Split(field, ".")
		current := interface{}(data)
		for _, part := range parts {
			if m, ok := current.(map[string]interface{}); ok {
				val, exists := m[part]
				if !exists {
					return nil
				}
				current = val
			} else {
				return nil
			}
		}
		return current
	}

	// Use parser for complex paths
	path, err := ParseFieldPath(field)
	if err != nil {
		return nil
	}
	return path.GetValue(data)
}

