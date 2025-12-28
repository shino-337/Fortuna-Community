# Fortuna - Detailed Logic Flow Documentation

**Date**: December 22, 2024  
**Purpose**: Detailed step-by-step logic flow for all components

---

## 📋 Table of Contents

1. [SBOM Pipeline Flow](#sbom-pipeline-flow)
2. [CVE Matching Flow](#cve-matching-flow)
3. [Risk Scoring Flow](#risk-scoring-flow)
4. [Policy Evaluation Flow](#policy-evaluation-flow)
5. [Insight Management Flow](#insight-management-flow)
6. [API Request Flow](#api-request-flow)

---

## 🔄 SBOM Pipeline Flow

### Step-by-Step: Pod → SBOM → Database

```
┌─────────────────────────────────────────────────────────────┐
│ STEP 1: Pod Creation in Kubernetes                          │
└─────────────────────────────────────────────────────────────┘
│
│ kubectl apply -f pod.yaml
│   └─> Kubernetes API Server creates Pod
│       └─> Pod scheduled to Node-X
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 2: Agent Detection (DaemonSet on Node-X)               │
└─────────────────────────────────────────────────────────────┘
│
│ Agent Pod (DaemonSet)
│   ├─> K8s Informer watches pods
│   │   └─> Field selector: spec.nodeName = Node-X
│   │
│   ├─> Pod event received: ADDED
│   │   └─> pod_watcher_local.go: OnAdd()
│   │
│   └─> Verify pod is on local node
│       └─> Safety check: pod.Spec.NodeName == agent.nodeName
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 3: SBOM Extraction (Agent)                            │
└─────────────────────────────────────────────────────────────┘
│
│ processor.go: ProcessPod()
│   │
│   ├─> For each container in pod:
│   │   │
│   │   ├─> Extract SBOM
│   │   │   └─> extractor.ExtractSBOM(imageRef)
│   │   │       │
│   │   │       ├─> Access docker socket (/var/run/docker.sock)
│   │   │       │
│   │   │       ├─> Pull image (if not local)
│   │   │       │
│   │   │       ├─> Extract image layers
│   │   │       │   └─> Parse package managers:
│   │   │       │       - dpkg (Debian/Ubuntu)
│   │   │       │       - rpm (RHEL/CentOS)
│   │   │       │       - apk (Alpine)
│   │   │       │       - pip (Python)
│   │   │       │       - npm (Node.js)
│   │   │       │       - go.mod (Go)
│   │   │       │
│   │   │       └─> Generate PURLs
│   │   │           └─> Format: pkg:ecosystem/name@version
│   │   │
│   │   ├─> Convert to proto
│   │   │   └─> convertToProto(pod, container, rawSBOM)
│   │   │       └─> pb.SBOMFinding{
│   │   │           AgentId: agentID,
│   │   │           NodeId: nodeID,
│   │   │           PodUid: pod.UID,
│   │   │           PodName: pod.Name,
│   │   │           Namespace: pod.Namespace,
│   │   │           ContainerName: container.Name,
│   │   │           ImageName: container.Image,
│   │   │           ImageDigest: imageDigest,
│   │   │           Packages: []Package{...}
│   │   │       }
│   │   │
│   │   └─> Send to Core
│   │       └─> grpcClient.SendSBOMFinding(ctx, sbomFinding)
│   │
│   └─> Wait for response
│       └─> resp.Success == true
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 4: Core Receives SBOM (gRPC Handler)                  │
└─────────────────────────────────────────────────────────────┘
│
│ handler_sbom.go: SendSBOMFinding()
│   │
│   ├─> Validate request
│   │   └─> Check required fields
│   │
│   ├─> Start database transaction
│   │   └─> tx := db.Begin()
│   │
│   ├─> Insert SBOM record
│   │   └─> tx.Create(&models.SBOM{
│   │       PodUID: req.PodUid,
│   │       PodName: req.PodName,
│   │       Namespace: req.Namespace,
│   │       ContainerName: req.ContainerName,
│   │       ImageDigest: req.ImageDigest,
│   │       AgentID: req.AgentId,
│   │       NodeID: req.NodeId,
│   │       GeneratedAt: req.GeneratedAt.AsTime(),
│   │   })
│   │
│   ├─> Insert SBOM components
│   │   └─> For each package in req.Packages:
│   │       └─> tx.Create(&models.SBOMComponent{
│   │           SBOMID: sbom.ID,
│   │           ComponentType: mapComponentType(pkg.Type),
│   │           ComponentName: pkg.Name,
│   │           ComponentVersion: pkg.Version,
│   │           PURL: generatePURL(pkg),
│   │           Licenses: pkg.Licenses,
│   │           Source: pkg.Source,
│   │       })
│   │
│   ├─> Commit transaction
│   │   └─> tx.Commit()
│   │
│   └─> Publish SBOM_CREATED event
│       └─> natsClient.Publish("ksam.sbom.created", event)
│
└─────────────────────────────────────────────────────────────┘
```

**Key Files**:
- `agent/internal/sbom/processor.go` - Lines 43-96
- `agent/pkg/sbom/extractor/` - SBOM extraction
- `core/internal/grpc/handler_sbom.go` - Lines 36-94
- `core/pkg/sbom/events.go` - Event definition

---

## 🔍 CVE Matching Flow

### Step-by-Step: SBOM → CVE Matches → Insights

```
┌─────────────────────────────────────────────────────────────┐
│ STEP 1: NATS Event Published                                │
└─────────────────────────────────────────────────────────────┘
│
│ Core publishes: "ksam.sbom.created"
│   └─> Event payload:
│       {
│         "type": "sbom.created",
│         "sbom_id": 123,
│         "pod_uid": "abc-123",
│         "pod_name": "my-pod",
│         "pod_namespace": "default",
│         "container_name": "app",
│         "image_digest": "sha256:...",
│         "timestamp": 1234567890
│       }
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 2: CVEMatcherWorker Receives Event                     │
└─────────────────────────────────────────────────────────────┘
│
│ cve_matcher_worker.go: Process()
│   │
│   ├─> Unmarshal event
│   │   └─> json.Unmarshal(msg.Data, &ev)
│   │
│   ├─> Load SBOM from database
│   │   └─> db.Where("id = ?", ev.SBOMID).First(&sbom)
│   │       └─> Verify SBOM exists (may have been deleted)
│   │
│   └─> Call matcher
│       └─> matcher.MatchSBOM(ctx, &sbom)
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 3: CVE Matching Logic                                  │
└─────────────────────────────────────────────────────────────┘
│
│ matcher.go: MatchSBOM()
│   │
│   ├─> Load SBOM components
│   │   └─> db.Where("sbom_id = ?", sbom.ID).Find(&components)
│   │
│   ├─> For each component:
│   │   │
│   │   ├─> Parse PURL
│   │   │   └─> ParsePURL(component.PURL)
│   │   │       └─> Extract: ecosystem, name, version
│   │   │
│   │   ├─> Normalize ecosystem
│   │   │   └─> normalizeQueryEcosystem(purl)
│   │   │       └─> Map to DB values:
│   │   │           - "deb" → "debian"
│   │   │           - "rpm" → "rhel"
│   │   │           - "apk" → "alpine"
│   │   │           - "pypi" → "pypi"
│   │   │           - "npm" → "npm"
│   │   │           - "golang" → "go"
│   │   │
│   │   ├─> Query CVE database
│   │   │   └─> dbManager.GetVulnerabilitiesForPackage(
│   │   │       ctx,
│   │   │       ecosystem,
│   │   │       packageName,
│   │   │       version
│   │   │   )
│   │   │       └─> SQL Query:
│   │   │           SELECT c.*, pv.version_constraint, pv.fixed_version
│   │   │           FROM package_vulnerabilities pv
│   │   │           JOIN cves c ON pv.cve_id = c.cve_id
│   │   │           WHERE pv.ecosystem = ?
│   │   │           AND pv.package_name = ?
│   │   │
│   │   ├─> For each CVE found:
│   │   │   │
│   │   │   ├─> Check version constraint
│   │   │   │   └─> IsVulnerable(version, constraint, ecosystem)
│   │   │   │       └─> Semver comparison:
│   │   │   │           - Parse constraint: "< 2.0.0"
│   │   │   │           - Compare: version < 2.0.0
│   │   │   │           - Return: true if vulnerable
│   │   │   │
│   │   │   └─> If vulnerable:
│   │   │       └─> Create CVEMatch
│   │   │           └─> &models.CVEMatch{
│   │   │               SBOMID: sbom.ID,
│   │   │               PodUID: sbom.PodUID,
│   │   │               ContainerName: sbom.ContainerName,
│   │   │               CVEID: cveData.ID,
│   │   │               PackageName: component.ComponentName,
│   │   │               PackageVersion: component.ComponentVersion,
│   │   │               PURL: component.PURL,
│   │   │               Severity: cveData.Severity,
│   │   │               CVSS: cveData.CVSSScore,
│   │   │               FixedVersion: cveData.FixedVersion,
│   │   │           }
│   │   │
│   │   └─> Append to matches array
│   │
│   └─> Return matches
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 4: Persist CVE Matches                                 │
└─────────────────────────────────────────────────────────────┘
│
│ cve_matcher_worker.go: persistMatches()
│   │
│   ├─> For each match:
│   │   │
│   │   ├─> Check for duplicate
│   │   │   └─> db.Where("sbom_id = ? AND cve_id = ? AND package_name = ?",
│   │   │       match.SBOMID, match.CVEID, match.PackageName)
│   │   │       .First(&existing)
│   │   │
│   │   └─> If not duplicate:
│   │       └─> db.Create(&match)
│   │
│   └─> Create insights (CRITICAL/HIGH only)
│       └─> For each match with severity CRITICAL or HIGH:
│           └─> insightMgr.CreateOrUpdateInsight(&insight)
│
└─────────────────────────────────────────────────────────────┘
```

**Key Files**:
- `core/pkg/worker/cve_matcher_worker.go` - Lines 51-87
- `core/pkg/cve/matcher/matcher.go` - Lines 36-122
- `core/pkg/cve/db_manager.go` - Database queries
- `core/pkg/cve/version/constraint.go` - Version comparison

---

## 📊 Risk Scoring Flow

### Step-by-Step: Insights → Risk Score

```
┌─────────────────────────────────────────────────────────────┐
│ STEP 1: Trigger Risk Calculation                            │
└─────────────────────────────────────────────────────────────┘
│
│ Triggers:
│   ├─> Insight created/updated
│   ├─> Scheduled evaluation (every 6 hours)
│   └─> API call: POST /api/v1/risk/scores/:uid/calculate
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 2: Load Insights for Resource                          │
└─────────────────────────────────────────────────────────────┘
│
│ scorer.go: CalculateScore(resourceUID)
│   │
│   ├─> Query active insights
│   │   └─> db.Where("affected_resources @> ?::jsonb AND status IN (?)",
│   │       uidJSON, []string{"active", "acknowledged"})
│   │       .Find(&insights)
│   │       └─> JSONB query (uses GIN index, very fast)
│   │
│   └─> If no insights:
│       └─> Return zero score (PriorityLevel: P4)
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 3: Calculate Base Score (0-40)                        │
└─────────────────────────────────────────────────────────────┘
│
│ scorer.go: calculateBaseScore()
│   │
│   ├─> Separate CVE and policy insights
│   │   └─> separateInsights(insights)
│   │
│   ├─> Calculate CVE base score
│   │   └─> calculateCVEBaseScore(cveInsights)
│   │       └─> For each CVE insight:
│   │           ├─> CRITICAL: 30-40 points
│   │           ├─> HIGH: 20-30 points
│   │           ├─> MEDIUM: 10-20 points
│   │           └─> LOW: 0-10 points
│   │       └─> Sum with diminishing returns
│   │
│   ├─> Calculate policy base score
│   │   └─> calculateBaseScore(policyInsights)
│   │       └─> Similar to CVE scoring
│   │
│   └─> Combine (cap at 40)
│       └─> baseScore = min(cveBaseScore + policyBaseScore, 40)
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 4: Calculate Exploitability Score (0-30)                │
└─────────────────────────────────────────────────────────────┘
│
│ scorer.go: calculateExploitabilityScore()
│   │
│   ├─> Public exploit available: +15
│   │   └─> Check CVE metadata for exploit references
│   │
│   ├─> Network accessible: +10
│   │   └─> Check if pod has service/ingress
│   │
│   ├─> Privilege escalation: +5
│   │   └─> Check if pod runs as root or has capabilities
│   │
│   └─> Recent CVE (< 30 days): +5
│       └─> Check CVE published date
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 5: Calculate Business Impact Score (0-30)                │
└─────────────────────────────────────────────────────────────┘
│
│ scorer.go: calculateBusinessImpactScore()
│   │
│   ├─> Production namespace: +15
│   │   └─> Check namespace labels/annotations
│   │
│   ├─> Critical workload: +10
│   │   └─> Check pod labels (app.kubernetes.io/tier: critical)
│   │
│   ├─> High resource count: +5
│   │   └─> Count related resources (deployments, services)
│   │
│   └─> External exposure: +10
│       └─> Check for ingress/LoadBalancer service
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 6: Calculate Time Decay (0.7-1.0)                      │
└─────────────────────────────────────────────────────────────┘
│
│ scorer.go: calculateTimeDecay()
│   │
│   ├─> Find oldest insight
│   │   └─> min(insights.DetectedAt)
│   │
│   └─> Calculate decay factor:
│       ├─> < 7 days: 1.0 (no decay)
│       ├─> 7-30 days: 0.9
│       ├─> 30-90 days: 0.8
│       └─> > 90 days: 0.7
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 7: Calculate Total Score & Priority                    │
└─────────────────────────────────────────────────────────────┘
│
│ scorer.go: CalculateScore()
│   │
│   ├─> Calculate total score
│   │   └─> totalScore = (baseScore + exploitScore + businessScore) × timeDecay
│   │       └─> Cap at 100
│   │
│   ├─> Determine priority level
│   │   └─> determinePriority(totalScore)
│   │       ├─> P0: 90-100 (Critical)
│   │       ├─> P1: 70-89 (High)
│   │       ├─> P2: 50-69 (Medium)
│   │       ├─> P3: 30-49 (Low)
│   │       └─> P4: 0-29 (Minimal)
│   │
│   └─> Store risk score
│       └─> db.CreateOrUpdate(&models.RiskScore{
│           ResourceUID: resourceUID,
│           TotalScore: totalScore,
│           BaseScore: baseScore,
│           ExploitabilityScore: exploitScore,
│           BusinessImpactScore: businessScore,
│           TimeDecay: timeDecay,
│           PriorityLevel: priorityLevel,
│           ScorerVersion: "v2",
│       })
│
└─────────────────────────────────────────────────────────────┘
```

**Key Files**:
- `core/pkg/risk/scorer.go` - Lines 48-141
- `core/pkg/worker/historical_risk_evaluator.go` - Scheduled scoring
- `core/pkg/models/risk_score.go` - Risk score model

---

## 🎯 Policy Evaluation Flow

### Step-by-Step: Resource → Policy Evaluation → Insight

```
┌─────────────────────────────────────────────────────────────┐
│ STEP 1: Resource Created/Updated                             │
└─────────────────────────────────────────────────────────────┘
│
│ Kubernetes API Server
│   └─> Resource event (CREATE/UPDATE)
│       └─> NormalizerWorker processes
│           └─> Publishes to "ksam.normalized.>"
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 2: Policy Worker Receives Event                        │
└─────────────────────────────────────────────────────────────┘
│
│ risk_worker.go: Process()
│   │
│   ├─> Unmarshal normalized data
│   │   └─> json.Unmarshal(msg.Data, &normalizedData)
│   │
│   └─> Call risk engine
│       └─> riskEngine.EvaluateResource(ctx, kind, normalizedData)
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 3: Policy Evaluation                                    │
└─────────────────────────────────────────────────────────────┘
│
│ riskengine/engine.go: EvaluateResource()
│   │
│   ├─> Enrich resource data
│   │   └─> enrichResourceData(resourceData)
│   │       └─> Parse raw_json and merge
│   │
│   ├─> Get applicable rules
│   │   └─> getApplicableRules(resourceType)
│   │       └─> Load from database/config
│   │
│   ├─> For each rule:
│   │   │
│   │   ├─> Check if enabled
│   │   │   └─> if !rule.Enabled: continue
│   │   │
│   │   ├─> Evaluate rule
│   │   │   └─> evaluateRule(ctx, rule, enrichedData)
│   │   │       └─> For each condition:
│   │   │           ├─> Parse CEL expression
│   │   │           ├─> Evaluate against resource
│   │   │           └─> Return: matched, score, error
│   │   │
│   │   └─> If matched:
│   │       └─> Create insight
│   │           └─> createInsight(rule, resourceType, enrichedData, score)
│   │
│   └─> Return insights
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 4: Admission Webhook (if enabled)                       │
└─────────────────────────────────────────────────────────────┘
│
│ webhook/handler.go: Validate()
│   │
│   ├─> Load policy instances
│   │   └─> db.Where("enabled = true AND namespace = ?", namespace)
│   │
│   ├─> For each policy instance:
│   │   │
│   │   ├─> Evaluate policy
│   │   │   └─> evaluator.EvaluateFast(ctx, resource)
│   │   │
│   │   └─> If violation:
│   │       └─> Check action:
│   │           ├─> "deny": Return admission denied
│   │           ├─> "alert": Allow but create insight
│   │           └─> "warn": Allow with warning
│   │
│   └─> Return admission response
│
└─────────────────────────────────────────────────────────────┘
```

**Key Files**:
- `core/pkg/worker/risk_worker.go` - Lines 69-116
- `core/pkg/riskengine/engine.go` - Lines 75-104
- `core/pkg/policy/evaluator.go` - CEL evaluation
- `core/internal/webhook/` - Admission webhook

---

## 📝 Insight Management Flow

### Step-by-Step: Insight Creation → Status Updates

```
┌─────────────────────────────────────────────────────────────┐
│ STEP 1: Insight Creation                                     │
└─────────────────────────────────────────────────────────────┘
│
│ Sources:
│   ├─> CVE Matcher Worker (vulnerability insights)
│   ├─> Risk Worker (policy violation insights)
│   └─> Manual creation via API
│
│ insight_manager.go: CreateOrUpdateInsight()
│   │
│   ├─> Check for existing insight
│   │   └─> db.Where("resource_uid = ? AND title = ?", ...)
│   │
│   ├─> If exists:
│   │   └─> Update (increment count, update timestamp)
│   │
│   └─> If new:
│       └─> db.Create(&models.Insight{
│           ResourceType: "Pod",
│           ResourceUID: podUID,
│           ResourceName: podName,
│           ResourceNamespace: namespace,
│           InsightType: "vulnerability" | "policy_violation",
│           Title: "CVE-2024-1234" | "Policy: No root containers",
│           Description: "...",
│           Severity: "CRITICAL" | "HIGH" | "MEDIUM" | "LOW",
│           Status: "active",
│           AffectedResources: JSONB[{"uid": "...", "type": "Pod"}],
│           Metadata: JSONB{...},
│       })
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 2: Insight Status Updates                               │
└─────────────────────────────────────────────────────────────┘
│
│ insight_status_updater.go: Process()
│   │
│   ├─> Scheduled job (every 5 minutes)
│   │
│   ├─> Query insights with status changes
│   │   └─> Check if resource still exists
│   │       └─> If resource deleted: status = "resolved"
│   │
│   └─> Update insights
│       └─> db.Model(&insight).Update("status", newStatus)
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 3: Insight Cleanup (TTL)                                │
└─────────────────────────────────────────────────────────────┘
│
│ scheduler.NewInsightsCleanupJob()
│   │
│   ├─> Runs every 24 hours
│   │
│   ├─> Query old insights
│   │   └─> db.Where("status = 'resolved' AND updated_at < ?", cutoff)
│   │       └─> cutoff = now() - 90 days
│   │
│   └─> Soft delete
│       └─> db.Delete(&insight) // GORM soft delete
│
└─────────────────────────────────────────────────────────────┘
```

**Key Files**:
- `core/pkg/insight/manager.go` - Insight management
- `core/pkg/worker/insight_status_updater.go` - Status updates
- `core/internal/scheduler/insights_cleanup.go` - TTL cleanup

---

## 🌐 API Request Flow

### Step-by-Step: HTTP Request → Response

```
┌─────────────────────────────────────────────────────────────┐
│ STEP 1: HTTP Request Received                               │
└─────────────────────────────────────────────────────────────┘
│
│ Client → Core API (port 8080)
│   └─> GET /api/v1/insights?severity=CRITICAL
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 2: Middleware Chain                                     │
└─────────────────────────────────────────────────────────────┘
│
│ routes.go: SetupRoutes()
│   │
│   ├─> CORS middleware (if enabled)
│   │   └─> Handle preflight requests
│   │
│   ├─> Auth middleware (if enabled)
│   │   └─> middleware.AuthMiddleware()
│   │       ├─> Extract JWT token from header
│   │       ├─> Validate token
│   │       └─> Set user context
│   │
│   └─> Logger middleware
│       └─> Log request details
│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ STEP 3: Route Handler                                        │
└─────────────────────────────────────────────────────────────┘
│
│ handlers.go: GetInsights()
│   │
│   ├─> Parse query parameters
│   │   └─> severity, namespace, status, etc.
│   │
│   ├─> Build database query
│   │   └─> db.Where("severity = ?", severity)
│   │       .Where("namespace = ?", namespace)
│   │       .Where("status = ?", status)
│   │       .Order("created_at DESC")
│   │       .Limit(limit)
│   │       .Offset(offset)
│   │
│   ├─> Execute query
│   │   └─> db.Find(&insights)
│   │
│   ├─> Format response
│   │   └─> JSON response with insights array
│   │
│   └─> Return response
│       └─> c.JSON(200, response)
│
└─────────────────────────────────────────────────────────────┘
```

**Key Files**:
- `core/internal/api/routes.go` - Route setup
- `core/internal/api/handlers.go` - Request handlers
- `core/internal/middleware/auth.go` - Authentication

---

## 🔄 Complete End-to-End Flow

### Pod Creation → Dashboard Display

```
1. Pod Created
   └─> Agent detects (5-10s)
       └─> Extract SBOM (2-5s)
           └─> Send to Core (mTLS gRPC)
               └─> Core stores SBOM (PostgreSQL)
                   └─> Publish SBOM_CREATED (NATS)
                       └─> CVEMatcherWorker processes (5-15s)
                           └─> Match CVEs (database query)
                               └─> Create CVEMatches
                                   └─> Create Insights (CRITICAL/HIGH)
                                       └─> Risk Scorer calculates (100ms)
                                           └─> Store RiskScore
                                               └─> Dashboard queries API
                                                   └─> Display insights & risk scores

Total Time: ~15-30 seconds
```

---

## 📊 Performance Characteristics

### Latency Breakdown

| Stage | Time | Notes |
|-------|------|-------|
| Pod creation → Agent detection | 1-2s | K8s informer latency |
| SBOM extraction | 2-5s | Depends on image size |
| gRPC transmission | <100ms | Local network |
| Database write | <50ms | PostgreSQL |
| NATS publish | <10ms | Local |
| CVE matching | 5-15s | Depends on component count |
| Insight creation | <100ms | Database write |
| Risk scoring | <100ms | Calculation + DB write |
| **Total** | **15-30s** | End-to-end |

### Throughput

- **SBOM generation**: ~10-20 pods/minute per agent
- **CVE matching**: ~50-100 SBOMs/minute per worker
- **Risk scoring**: ~1000 resources/minute
- **API requests**: ~1000 req/s (p99 < 50ms)

---

## ✅ Flow Documentation Complete

This document provides detailed step-by-step logic flows for:
- ✅ SBOM Pipeline (Pod → SBOM → Database)
- ✅ CVE Matching (SBOM → CVE Matches → Insights)
- ✅ Risk Scoring (Insights → Risk Score)
- ✅ Policy Evaluation (Resource → Policy → Insight)
- ✅ Insight Management (Creation → Updates → Cleanup)
- ✅ API Request Flow (HTTP → Handler → Response)

**Ready for implementation and debugging!** 🚀

---

*Fortuna K8s Management Platform - Detailed Logic Flow v1.0*



