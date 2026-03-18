package osv

import (
	"encoding/json"
	"fmt"
)

func ParseDocument(data []byte) (*Document, error) {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse OSV JSON: %w", err)
	}
	if doc.ID == "" {
		return nil, fmt.Errorf("parse OSV JSON: missing id")
	}
	return &doc, nil
}

