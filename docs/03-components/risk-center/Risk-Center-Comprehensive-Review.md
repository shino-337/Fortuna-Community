# Fortuna – Risk Center Comprehensive Review
Cảm ơn bạn đã cung cấp tài liệu chi tiết Risk-Center-Overview-Answer.md. Tôi đã phân tích toàn bộ luồng xử lý, logic kinh doanh, UI/UX, các cấu phần liên quan và cơ chế hệ thống dựa trên thông tin bạn cung cấp (NATS, WebSocket, cache, DB schema, React components, RiskWorker, InsightManager, risk_rules DB, PCE từ pod_capabilities, audit_logs, export CSV/PDF…).
Dưới đây là đánh giá tổng thể với 10 findings quan trọng nhất, sắp xếp theo mức độ ưu tiên (Critical → Low). Mỗi finding được trình bày đầy đủ theo định dạng chuẩn, kèm Technical Root Cause, Impact, Risk Level và Recommendation cụ thể (có luồng thực thi, code gợi ý và effort estimate).

Finding #1: WebSocket Real-time & NATS Broadcast Hoàn Hảo Nhưng Chưa Có Delta Update
Description
Hệ thống đã implement real-time rất tốt (NATS fortuna.insights.updated → BroadcastRisksUpdate → WebSocket /ws/risks → Dashboard refetch), nhưng chỉ gửi message “insights_updated” mà không kèm delta (new/updated/deleted insights) → client vẫn phải gọi lại toàn bộ GET /risk/insights.
Technical Root Cause
RisksWSHub chỉ broadcast JSON {"type":"insights_updated"} (không payload); Dashboard luôn refetch full list + summary (fetchData).
Impact
Với cluster lớn (hàng nghìn findings), refetch full list mỗi khi có 1 risk mới gây latency không cần thiết và tăng load API/DB.
Risk Level
High
Recommendation

Effort: Low (1-2 ngày).
Luồng thực thi:
Khi BatchCreateOrUpdateInsights → publish NATS fortuna.insights.updated kèm payload delta (ids changed, type: create/update/delete).
Subscriber gọi BroadcastRisksUpdate(delta).
WS gửi full delta object.
React dùng useSWR hoặc Zustand mutate cache thay vì refetch toàn bộ.

Code gợi ý:Go// risks_ws_hub.go
type UpdatePayload struct { Type string; ChangedIDs []string; Severity string }
defaultRisksHub.Broadcast(json.Marshal(UpdatePayload{Type: "insights_updated", ChangedIDs: changedIDs}))
Test: Simulate 100 updates/giây → verify client chỉ update rows affected.

Finding #2: Risk Scoring Chưa Unified – Insights & risk_scores Vẫn Riêng Biệt
Description
Risk Center hiển thị cả severity (từ insights) và total_score/priority_level (từ risk_scores), nhưng hai bảng vẫn tách rời; PCE không đóng góp vào công thức chung.
Technical Root Cause
InsightManager tạo insights; risk score tính riêng (POST /risk/scores/:uid/calculate hoặc job); chỉ join khi withScores=1.
Impact
User thấy inconsistency giữa severity bar và cột Risk Score; PCE exposure không ảnh hưởng priority → khó prioritize chính xác.
Risk Level
High
Recommendation

Effort: Medium (3-5 ngày).
Luồng thực thi:
Refactor createInsight → luôn gọi scorer.CalculateScore() và lưu total_score, priority_level trực tiếp vào insights table (thêm cột).
Xóa join risk_scores ở GET /risk/insights.
PCE evaluator thêm contribution (e.g., +15% nếu capability severity = Critical).

Migration: ALTER TABLE insights ADD COLUMN total_score, priority_level.

Finding #3: MemoryRisksCache (TTL 60s) Chưa Invalidate Khi Update
Description
Cache key dựa trên filter nhưng không invalidate khi có new insights (chỉ dựa WebSocket refetch).
Technical Root Cause
defaultRisksCache.Set(key, data, 60s) mà không có cơ chế clear khi BroadcastRisksUpdate.
Impact
User có thể thấy data cũ đến 60s dù đã có critical finding mới.
Risk Level
Medium-High
Recommendation

Effort: Low.
Khi publish fortuna.insights.updated → gọi cache.ClearByPrefix("insights:list:") hoặc publish Redis PUB/SUB để clear cache (nếu migrate sang Redis sau).

Finding #4: Export CSV/PDF Chưa Có Pagination Hoặc Streaming
Description
Export limit 10.000 dòng nhưng không streaming; query full DB trong một lần.
Technical Root Cause
GET /risk/insights/export gọi getInsightsListData với pageSize=10000.
Impact
Với >10k findings → memory spike hoặc timeout.
Risk Level
Medium
Recommendation

Effort: Low.
Chuyển sang streaming CSV (gin.Context.StreamWriter) hoặc worker background job xuất file → download link.

Finding #5: PCE Không Có State Machine Và Stale Detection
Description
PCE chỉ là bảng pod_capabilities sync từ Agent; không có trạng thái Detected→Confirmed→Exploited và không stale detection.
Technical Root Cause
PCE evaluator hiện chỉ sync và tính severity; không logic stale như InsightsCleanupJob.
Impact
PCE exposure có thể hiển thị capability đã không còn tồn tại (pod deleted).
Risk Level
Medium
Recommendation

Effort: Medium.
Thêm cột state + last_seen_at trong pod_capabilities; thêm PCECleanupJob giống InsightsCleanupJob.

Finding #6: Evidence (jsonb) Chưa Mask Sensitive Data
Description
Cột evidence và violated_rules có thể chứa command line, tokens, process args mà không mask.
Technical Root Cause
Không có sanitization khi createInsight hoặc trong handlers.
Impact
Rủi ro lộ thông tin nhạy cảm khi user xem drawer.
Risk Level
Medium
Recommendation

Thêm sanitizer function trước khi lưu jsonb (mask fields chứa “password”, “token”, “key”).

Finding #7: Bulk Actions Chỉ Hỗ Trợ Từng Insight Một
Description
Acknowledge/Resolve/Dismiss chỉ có API single; không bulk.
Technical Root Cause
Chưa có endpoint POST /risk/insights/bulk.
Impact
SecOps mất thời gian khi xử lý hàng trăm findings.
Risk Level
Medium
Recommendation

Thêm endpoint bulk (array of ids) với transaction.

Finding #8: Observability Thiếu Histogram Latency Và Alerting Rules
Description
Chỉ có counter insights_created_total; không histogram correlation_duration, risk_queries_total.
Technical Root Cause
Chưa instrument đầy đủ promhttp cho risk pipeline.
Impact
Khó phát hiện bottleneck (ví dụ slow EvaluateResource).
Risk Level
Medium
Recommendation

Thêm histogram fortuna_risk_evaluation_duration_seconds và fortuna_insights_batch_size.

Finding #9: Rule Engine Hỗ Trợ UI Edit Nhưng Chưa Có Versioning
Description
User có thể edit risk_rules qua Settings, nhưng không lưu history/version.
Technical Root Cause
Bảng risk_rules không có version hoặc audit trail đầy đủ.
Impact
Không trace được ai thay đổi rule khi xảy ra false-positive.
Risk Level
Low
Recommendation

Thêm cột version + created_by; lưu snapshot cũ vào bảng risk_rules_history.

Finding #10: WebSocket Chưa Có Authentication & Rate Limiting
Description
WS endpoint /ws/risks chưa thấy auth middleware rõ ràng (chỉ JWT ở HTTP).
Technical Root Cause
RisksWSHub chưa wrap middleware Auth.
Impact
Rủi ro unauthorized access nếu token expire hoặc leak.
Risk Level
Low (nhưng cần fix sớm)
Recommendation

Wrap WS với JWT validation giống HTTP middleware.


Tổng kết & Roadmap đề xuất
Điểm mạnh nổi bật:

Real-time WebSocket + NATS broadcast rất tốt
Cache MemoryRisksCache + Export CSV/PDF
Rule engine có UI edit
Audit log đầy đủ
PCE có heatmap & trend chart

Ưu tiên fix ngay (1-2 sprint):

Delta WebSocket (Finding #1)
Unified scoring (Finding #2)
Cache invalidation (Finding #3)

Ưu tiên trung bình (3-4 sprint):
PCE state machine, bulk actions, evidence masking, observability histogram.
Tổng rủi ro hiện tại: Medium. Với các cải tiến trên, Risk Center sẽ đạt mức enterprise-grade (tương đương Prisma Cloud hoặc Sysdig Secure).