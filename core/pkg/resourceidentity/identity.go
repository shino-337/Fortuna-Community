package resourceidentity

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var ErrInvalid = errors.New("invalid cluster-qualified resource identity")

// Identity is the canonical identity for a cluster-owned Kubernetes resource.
// Resource UIDs are not treated as globally unique across clusters.
type Identity struct {
	ClusterID   string
	ResourceUID string
}

// New validates an exact cluster/resource pair. Callers must not silently trim or
// otherwise normalize authorization keys because doing so can change ownership.
func New(clusterID, resourceUID string) (Identity, error) {
	id := Identity{ClusterID: clusterID, ResourceUID: resourceUID}
	if err := id.Validate(); err != nil {
		return Identity{}, err
	}
	return id, nil
}

func (i Identity) Validate() error {
	if i.ClusterID == "" || i.ResourceUID == "" {
		return fmt.Errorf("%w: cluster_id and resource_uid are required", ErrInvalid)
	}
	if strings.TrimSpace(i.ClusterID) != i.ClusterID || strings.TrimSpace(i.ResourceUID) != i.ResourceUID {
		return fmt.Errorf("%w: ownership keys must not contain outer whitespace", ErrInvalid)
	}
	if containsControl(i.ClusterID) || containsControl(i.ResourceUID) {
		return fmt.Errorf("%w: ownership keys must not contain control characters", ErrInvalid)
	}
	return nil
}

func containsControl(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}

func (i Identity) Equal(other Identity) bool {
	return i.ClusterID == other.ClusterID && i.ResourceUID == other.ResourceUID
}

// Key returns a collision-safe in-process key. The length prefix prevents
// delimiter ambiguity without changing the underlying identity values.
func (i Identity) Key() (string, error) {
	if err := i.Validate(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%s:%s", len(i.ClusterID), i.ClusterID, i.ResourceUID), nil
}
