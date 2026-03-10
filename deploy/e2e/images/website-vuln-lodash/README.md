# Static website with vulnerable dependency (CVE scan target)

- **Web server:** `serve` (static files on port 3000).
- **Content:** HTML/CSS/JS in `public/`.
- **Vulnerable dependency:** `lodash@4.17.19` (e.g. CVE-2020-8203). Used for SBOM/CVE scan; agent extracts node_modules for SBOM, Core CVE matcher can produce vulnerability insights.

Build and run:

```bash
nerdctl --namespace k8s.io build -t website-vuln-lodash:latest -f deploy/e2e/images/website-vuln-lodash/Dockerfile deploy/e2e/images/website-vuln-lodash
kubectl apply -f deploy/e2e/website-vuln-lodash-pod.yaml
```

Pod runs long-term (`restartPolicy: Always`, serve keeps running). Schedule on master with `nodeSelector` so the image is available.

## Pod Detail – Network connection testing

This pod is suitable for **Pod Detail network connection** E2E tests:

- The container runs `serve` listening on **TCP port 3000**, so `ss -tunap` / `netstat -tunap` inside the container shows at least one **LISTEN** socket. The agent’s network collector uses these tools, so this pod will have non-empty `pod_network_connections` once the Pod Detail reporter has run.
- **E2E script:** `scripts/e2e/test-pod-detail-lodash-network.sh` deploys (or reuses) the lodash pod, waits for the reporter, then asserts GET `/api/v1/pods/by-uid/:uid/network-connections` returns at least one item (expect LISTEN on port 3000).
- **Prerequisites:** Same as other Pod Detail tests: Agent on the same node as the pod (e.g. `nodeSelector: k8s-master`), image `website-vuln-lodash:latest` built and available.
