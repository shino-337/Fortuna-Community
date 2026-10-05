package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// QueryService is bound to one already-authorized cluster. Legacy global
// traversal and arbitrary Cypher entry points are deliberately unavailable.
type QueryService struct{ engine *AgeGraphEngine }

type PathConstraint struct {
	MaxDepth               int
	RequireRuntimeEvidence bool
	MinRealism             float64
	MustIncludeTechnique   string
}

func (s *QueryService) FindPaths(ctx context.Context, sourceUID, targetUID string, constraints PathConstraint) ([]AttackPath, error) {
	if constraints.RequireRuntimeEvidence || constraints.MinRealism != 0 || constraints.MustIncludeTechnique != "" {
		return nil, fmt.Errorf("requested AGE evidence constraint is unsupported")
	}
	if sourceUID == "" || targetUID == "" {
		return nil, fmt.Errorf("source and target UID required")
	}
	depth, err := boundedDepth(constraints.MaxDepth)
	if err != nil {
		return nil, err
	}
	return s.engine.paths(ctx, depth, "a.uid = $source AND b.uid = $target", map[string]any{"source": sourceUID, "target": targetUID})
}

// AGE annotates JSON values outside quoted strings. Remove only the three
// expected type suffixes; quoted property contents remain byte-for-byte intact.
func agePathJSON(raw string) ([]byte, error) {
	var out []byte
	quoted, escaped := false, false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if quoted {
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
			out = append(out, c)
			continue
		}
		if c == ':' && i+1 < len(raw) && raw[i+1] == ':' {
			matched := false
			for _, suffix := range []string{"::vertex", "::edge", "::path"} {
				if strings.HasPrefix(raw[i:], suffix) {
					i += len(suffix) - 1
					matched = true
					break
				}
			}
			if !matched {
				return nil, fmt.Errorf("unsupported AGE type annotation")
			}
			continue
		}
		out = append(out, c)
	}
	return out, nil
}
func decodeAGEPath(raw, clusterID string) (AttackPath, error) {
	body, err := agePathJSON(raw)
	if err != nil {
		return AttackPath{}, err
	}
	var data []struct {
		ID         json.Number    `json:"id"`
		Label      string         `json:"label"`
		Properties map[string]any `json:"properties"`
		Start      json.Number    `json:"start_id"`
		End        json.Number    `json:"end_id"`
	}
	if err = json.Unmarshal(body, &data); err != nil {
		return AttackPath{}, fmt.Errorf("decode AGE path: %w", err)
	}
	if len(data) < 3 || len(data)%2 != 1 {
		return AttackPath{}, fmt.Errorf("malformed AGE path")
	}
	path := AttackPath{Nodes: []PathNode{}, Edges: []PathEdge{}, Length: len(data) / 2}
	for i, item := range data {
		if item.Properties["cluster_id"] != clusterID {
			return AttackPath{}, fmt.Errorf("foreign AGE path vertex/edge")
		}
		if _, err = ageVertexID(item.ID.String()); err != nil {
			return AttackPath{}, err
		}
		if i%2 == 0 {
			path.Nodes = append(path.Nodes, PathNode{ID: item.ID.String(), Type: item.Label, Properties: item.Properties})
		} else {
			if item.Start.String() != data[i-1].ID.String() || item.End.String() != data[i+1].ID.String() {
				return AttackPath{}, fmt.Errorf("inconsistent AGE path endpoints")
			}
			path.Edges = append(path.Edges, PathEdge{Type: item.Label, Source: item.Start.String(), Target: item.End.String(), Properties: item.Properties})
		}
	}
	return path, nil
}
