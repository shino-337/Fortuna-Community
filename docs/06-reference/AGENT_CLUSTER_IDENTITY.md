# Agent cluster identity

Migration 150 adds an indexed cluster_id to agents. Existing rows remain
unassigned; node-name similarity is not used as migration evidence. Agent IDs
remain globally unique for compatibility with the existing gRPC contract.

The existing authenticated HTTP sync payload supplies the canonical cluster ID.
The identity upsert atomically claims an unassigned agent, updates an agent in the
same cluster, or returns 409 for an ID already assigned elsewhere. Database
failures return 500 rather than being ignored. Cluster agent lists and Dashboard
counts use explicit cluster_id; unassigned legacy agents are excluded until sync.

Legacy gRPC Register/Ping can create an unassigned agent and refresh it by agent
ID. A Ping without an agent ID never updates a row by node name. The existing
wire format is unchanged. This change prevents accidental cross-cluster node-name
correlation; it does not add per-cluster cryptographic attestation. Shared agent
credentials still belong to the existing ingestion trust model.

Deploy Core migrations before the updated handlers, then allow a successful HTTP
inventory sync from each agent. Verify cluster counts for two clusters with the
same node names. Do not backfill cluster_id by node name. For deliberate agent
reassignment, investigate the conflicting identity instead of automatically
moving it. Production PostgreSQL migration and live transport checks remain lab
gates. Keep the additive column if rolling back Core.
