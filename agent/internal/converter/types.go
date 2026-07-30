package converter

// InventoryItem represents a Kubernetes resource item for inventory sync.
// Used by the collector when streaming to Core (StreamInventory RPC not in current proto;
// agent uses HTTP syncer in main path; this type keeps converter/collector buildable).
type InventoryItem struct {
	Kind      string
	Uid       string
	Name      string
	Namespace string
	Labels    map[string]string
	RawJson   string
	Timestamp int64
}
