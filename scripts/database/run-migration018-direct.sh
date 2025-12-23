#!/bin/bash
set -e
POSTGRES_POD=$(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || kubectl get pods -n ksam | grep postgres | awk '{print $1}' | head -1)
echo "PostgreSQL pod: $POSTGRES_POD"
SQL="ALTER TABLE risk_scores ADD COLUMN IF NOT EXISTS exploitability_score DECIMAL(5,2) DEFAULT 0.0, ADD COLUMN IF NOT EXISTS business_impact_score DECIMAL(5,2) DEFAULT 0.0, ADD COLUMN IF NOT EXISTS scorer_version VARCHAR(10) DEFAULT 'v1';"
kubectl exec -n ksam $POSTGRES_POD -- psql -U ksam -d ksam -c "$SQL"
echo "Migration018 completed"

