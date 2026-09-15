# Workload and capability inventory scope

Deployment and ReplicaSet lists and details apply the caller's cluster allowlist
before reading, counting or pagination. Both cluster and clusterId filters are
accepted; conflicting filters return 400 and forbidden clusters return 403.
Details use positive database IDs (legacy route contract); unavailable or
out-of-scope details return 404. Pagination has a 1000-row maximum and rejects
integer-overflow effects by checking the requested page against the count first.

All capability list, summary and trend endpoints apply the same cluster scope
before aggregation. Aggregates describe active pods only. Missing capability
schema returns 503, not a successful empty inventory. Count/query errors return
500. Trends cover exactly the requested UTC calendar days, include zero days,
and count capability creation observations rather than daily active capability
snapshots. The portable timestamp path is covered by SQLite tests; PostgreSQL
and volume testing remain lab gates.
