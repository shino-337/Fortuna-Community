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
