package securityaudit

// AuditChainExtender is reserved for future hash-chained audit anchors (tamper-evidence).
type AuditChainExtender interface {
	Extend(chainState []byte, event Event) (newState []byte, err error)
}

// SignedAuditBatchProducer is reserved for future signed batch export (WORM / compliance bundles).
type SignedAuditBatchProducer interface {
	ProduceBatch(events []Event) (payload []byte, signature string, err error)
}

// WatermarkHook is reserved for export watermarking (DLP integration).
type WatermarkHook interface {
	ApplyWatermark(resourceType string, raw []byte) ([]byte, error)
}
