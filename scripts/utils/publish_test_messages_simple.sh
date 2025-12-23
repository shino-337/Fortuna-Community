#!/bin/bash

# Simple script to publish test messages to NATS using kubectl exec
# This script publishes JSON messages directly to NATS streams

set -e

NAMESPACE="ksam"

# Get NATS pod
NATS_POD=$(kubectl get pods -n "$NAMESPACE" -l app=nats -o jsonpath='{.items[0].metadata.name}')

if [ -z "$NATS_POD" ]; then
    echo "ERROR: NATS pod not found"
    exit 1
fi

echo "Publishing test messages to NATS..."
echo "NATS Pod: $NATS_POD"
echo ""

# Function to publish a message
publish_message() {
    local subject=$1
    local message=$2
    echo "Publishing to $subject: $message"
    
    # Use kubectl exec to run nats CLI or use a simple echo to NATS
    # If nats CLI is not available, we'll use a workaround
    kubectl exec -n "$NAMESPACE" "$NATS_POD" -- sh -c "echo '$message' | nc localhost 4222" 2>/dev/null || {
        # Alternative: Use nats-box if available
        echo "Trying alternative method..."
        kubectl run -n "$NAMESPACE" --rm -i --restart=Never nats-pub --image=natsio/nats-box --command -- sh -c "echo '$message' | nats pub '$subject'" 2>/dev/null || {
            echo "WARNING: Could not publish message directly"
            return 1
        }
    }
}

# Generate test messages
TIMESTAMP=$(date +%s)

# Test Pod messages
for i in {1..10}; do
    MSG="{\"kind\":\"Pod\",\"uid\":\"pod-uid-$i\",\"name\":\"test-pod-$i\",\"namespace\":\"default\",\"labels\":{\"cluster\":\"test\"},\"raw_json\":\"{}\",\"timestamp\":$TIMESTAMP}"
    kubectl run -n "$NAMESPACE" --rm -i --restart=Never nats-pub-$i --image=natsio/nats-box --command -- sh -c "echo '$MSG' | nats pub 'ksam.raw.pods'" 2>/dev/null || echo "Failed to publish pod-$i"
    sleep 0.1
done

# Test ServiceAccount messages
for i in {1..5}; do
    MSG="{\"kind\":\"ServiceAccount\",\"uid\":\"sa-uid-$i\",\"name\":\"test-sa-$i\",\"namespace\":\"default\",\"labels\":{\"cluster\":\"test\"},\"raw_json\":\"{}\",\"timestamp\":$TIMESTAMP}"
    kubectl run -n "$NAMESPACE" --rm -i --restart=Never nats-pub-sa-$i --image=natsio/nats-box --command -- sh -c "echo '$MSG' | nats pub 'ksam.raw.serviceaccounts'" 2>/dev/null || echo "Failed to publish sa-$i"
    sleep 0.1
done

# Test Role messages
for i in {1..3}; do
    MSG="{\"kind\":\"Role\",\"uid\":\"role-uid-$i\",\"name\":\"test-role-$i\",\"namespace\":\"default\",\"labels\":{\"cluster\":\"test\"},\"raw_json\":\"{}\",\"timestamp\":$TIMESTAMP}"
    kubectl run -n "$NAMESPACE" --rm -i --restart=Never nats-pub-role-$i --image=natsio/nats-box --command -- sh -c "echo '$MSG' | nats pub 'ksam.raw.roles'" 2>/dev/null || echo "Failed to publish role-$i"
    sleep 0.1
done

echo ""
echo "✅ Test messages published"

