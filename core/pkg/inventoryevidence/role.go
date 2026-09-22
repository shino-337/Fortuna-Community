package inventoryevidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func Key(kind, uid string) string { return kind + "/" + uid }

// RoleDigest binds the observed rules to the exact static resource projection.
// Canonical JSON avoids formatting-only changes invalidating observations.
func RoleDigest(clusterID, kind, uid, name, namespace, rules string) (string, error) {
	var policy interface{}
	if err := json.Unmarshal([]byte(rules), &policy); err != nil {
		return "", err
	}
	raw, err := json.Marshal([]interface{}{clusterID, kind, uid, name, namespace, policy})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}
