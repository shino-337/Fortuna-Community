# Component identity — matcher vs insight (INS-1)

## Khóa chính (canonical)

Trên worker CVE matcher, **join insight ↔ match** dùng:

- `CVEMatch.PackageName` **==** `SBOMComponent.ComponentName` (sau khi resolve snapshot/DB).

Matcher và persist match đều dùng tên package đã chuẩn hóa trong luồng matcher; insight build nhận `component` từ map `componentMap[match.PackageName]`.

## Snapshot vs DB

1. **Matching** dùng `componentsOverride` từ NATS `components_snapshot` khi có (tránh race soft-delete).
2. **Insight** trước đây chỉ load component từ **DB** theo `component_name IN (...)`.

Nếu có lệch hiếm (DB chưa kịp / tên khớp snapshot), insight có thể thiếu dòng.

**Hành vi đã siết:** sau khi load DB, worker **bổ sung** `componentMap` từ snapshot cho mọi `ComponentName` chưa có trong DB (code: `cve_matcher_worker.go`).

## Khuyến nghị vận hành

- Không dùng hai tên khác nhau cho cùng một gói trong cùng một SBOM (agent/core regeneration).
- Khi debug lệch: so sánh `PackageName` trên `cve_matches` với `component_name` trên `sbom_components` và field `name` trong snapshot event.

## Test / replay

Thêm replay test nếu thêm nhánh logic mới: cùng `sbom_id`, cùng `PackageName`, insight phải tạo được khi snapshot chứa component đó.
