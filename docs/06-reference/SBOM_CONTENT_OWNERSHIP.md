# SBOM content and workload ownership

An SBOM ID identifies one cluster/Pod/container/image-digest observation. It is
never reused for another workload. `content_id` references an immutable,
resource-independent package snapshot in `sbom_image_contents`. The content hash
covers the image digest, OS/Go metadata, packages and their trust/provenance, and
resolver/signature versions. Equal image digests alone do not imply equal content.
Pod names, namespaces, labels, agent IDs and cluster IDs are excluded from shared
content. Authorization and findings continue to use the observation, never a
shared content ID. Existing component rows remain the workload-local read
projection; content records are retained when an observation is soft-deleted.

## Upgrade

The fail-closed startup invariant runs after cluster ownership backfill. It adds
the nullable content reference and foreign key, checks active workload uniqueness,
installs PostgreSQL guards and backfills resolved active observations in batches
of 200. Each attachment is transactional and resumable. Unknown ownership stays
unlinked; the migration does not choose the first cluster, container or Pod.

Conflicting active observations or associations stop startup. The error names the
table/group count. Inspect the affected IDs and their components/findings before
explicitly reconciling them; the migration neither deletes evidence nor rewrites
ownership. For duplicate observations:

```sql
SELECT cluster_id, pod_uid, container_name, image_digest, array_agg(id ORDER BY id)
FROM sboms
WHERE deleted_at IS NULL AND cluster_id <> '' AND pod_uid <> ''
  AND container_name <> '' AND image_digest <> ''
GROUP BY cluster_id, pod_uid, container_name, image_digest
HAVING count(*) > 1;
```

Do not bypass the startup failure by disabling the guards. Preserve a database
backup before a populated upgrade. Index creation and backfill can block writes;
plan the rollout accordingly. Rolling back the binary does not remove the guards
or restore the retired combined RPC; an older writer that violates the identity
contract will fail. Certificate/Agent identity cutover remains a separate package.

## Verification

Named regressions cover shared content without shared ownership, provenance drift,
foreign events and scan links, missing-cluster rejection, and retired combined
writes. `TestSBOMConcurrentOwnershipPostgres` runs against PostgreSQL 16 in CI and
must pass rather than skip. It starts with populated pre-content storage, verifies
resumable backfill/quarantine, immutable content and owner guards, foreign
CVE/malware/scan rejection, concurrent first ingest and direct duplicate rejection.
Conflicting duplicate evidence is preserved when migration refuses startup.
The live two-cluster deployment gate remains in work package F.
