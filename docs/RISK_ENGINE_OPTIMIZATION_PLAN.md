# Risk Engine Optimization Plan

## Phân tích hiện trạng

### ✅ Điểm mạnh hiện tại
- Code structure sạch, tách biệt concerns
- Type-safe với Go structs
- Đã có basic condition system
- Đã có data enrichment

### ❌ Vấn đề cần giải quyết
1. **Hardcoded rules** - Cần rebuild để thay đổi
2. **Limited expression** - Chỉ string matching đơn giản
3. **Incomplete field access** - Không hỗ trợ deep nesting
4. **Weak duplicate detection** - Dùng LIKE trên JSON text
5. **No validation** - Rules có thể invalid nhưng không bị catch
6. **No versioning** - Không có version control cho rules

## Phương án tối ưu (Resource-Efficient)

### Nguyên tắc thiết kế
1. **Lazy Loading**: Chỉ load rules khi cần
2. **Compiled Rules**: Pre-compile expressions để tăng tốc
3. **Memory Efficient**: Cache minimal, clear khi không dùng
4. **Incremental**: Migrate từng phần, không phá vỡ hệ thống hiện tại
5. **Backward Compatible**: Giữ được hardcoded rules làm fallback

### Kiến trúc đề xuất

```
┌─────────────────────────────────────────────────────────┐
│              Hybrid Rule System                          │
│  ┌──────────────────┐  ┌──────────────────┐           │
│  │  YAML Rules      │  │  Hardcoded Rules │           │
│  │  (Optional)      │  │  (Fallback)      │           │
│  └────────┬─────────┘  └────────┬─────────┘           │
│           │                      │                      │
│           └──────────┬───────────┘                      │
│                      │                                   │
│                      ▼                                   │
│         ┌─────────────────────────┐                     │
│         │   Rule Registry         │                     │
│         │  • Lazy loading         │                     │
│         │  • Compiled cache       │                     │
│         │  • Memory efficient     │                     │
│         └────────────┬────────────┘                     │
│                      │                                   │
│                      ▼                                   │
│         ┌─────────────────────────┐                     │
│         │   Enhanced Engine       │                     │
│         │  • Lightweight CEL      │                     │
│         │  • Simple JSONPath      │                     │
│         │  • Field accessor       │                     │
│         └─────────────────────────┘                     │
└─────────────────────────────────────────────────────────┘
```

## Implementation Strategy

### Phase 1: Lightweight YAML Loader (Week 1)
**Mục tiêu**: Thêm khả năng load YAML rules nhưng không thay thế hoàn toàn

**Tối ưu**:
- ✅ Load rules on-demand (lazy)
- ✅ Cache compiled expressions
- ✅ Validate chỉ khi load
- ✅ Không dùng heavy dependencies (JSON Schema optional)
- ✅ Giữ hardcoded rules làm fallback

**Deliverables**:
- `pkg/riskengine/loader/yaml_loader.go` - Lightweight YAML loader
- `pkg/riskengine/loader/validator.go` - Simple validation (không dùng JSON Schema)
- `rules/` directory với YAML files
- Integration với Engine hiện tại

### Phase 2: Enhanced Field Access (Week 1-2)
**Mục tiêu**: Cải thiện field access nhưng không dùng full JSONPath

**Tối ưu**:
- ✅ Simple dot notation parser (a.b.c)
- ✅ Array indexing support (items[0], items[].field)
- ✅ Không dùng full JSONPath library (quá nặng)
- ✅ Cache parsed paths

**Deliverables**:
- Enhanced `getFieldValue()` function
- Path parser với caching
- Backward compatible

### Phase 3: Lightweight Expression Engine (Week 2)
**Mục tiêu**: Thêm CEL support nhưng optional, không bắt buộc

**Tối ưu**:
- ✅ CEL chỉ dùng khi rule có CEL expression
- ✅ Pre-compile expressions khi load rule
- ✅ Fallback về simple evaluation nếu CEL fail
- ✅ Không compile nếu không có CEL expressions

**Deliverables**:
- Optional CEL integration
- Expression compiler với caching
- Fallback mechanism

### Phase 4: Rule Registry & Management (Week 2-3)
**Mục tiêu**: Quản lý rules hiệu quả, không chiếm nhiều memory

**Tối ưu**:
- ✅ Rule registry với map[string]*Rule
- ✅ Lazy loading từ YAML
- ✅ Compiled expression cache (LRU cache)
- ✅ Rule metadata minimal
- ✅ Hot reload optional (không watch mặc định)

**Deliverables**:
- `pkg/riskengine/registry/rule_registry.go`
- LRU cache cho compiled expressions
- Hot reload API (manual trigger, không auto-watch)

## Resource Optimization Details

### Memory Management
1. **Rule Storage**: 
   - Store rules as pointers (không copy)
   - Use map[string]*Rule thay vì slice
   - Clear cache khi memory pressure

2. **Expression Compilation**:
   - Pre-compile khi load rule (một lần)
   - Cache compiled programs (LRU, max 100 rules)
   - Clear cache khi rule disabled

3. **Field Access**:
   - Cache parsed paths (map[string]Path)
   - Limit cache size (max 1000 paths)
   - Simple parser, không dùng regex heavy

### CPU Optimization
1. **Lazy Evaluation**:
   - Chỉ evaluate rules applicable cho resource type
   - Short-circuit evaluation (AND/OR)
   - Skip disabled rules early

2. **Compiled Expressions**:
   - Pre-compile CEL expressions
   - Cache compiled programs
   - Reuse compiled programs

3. **Minimal Dependencies**:
   - CEL: Chỉ import khi cần
   - JSONPath: Không dùng, tự implement simple parser
   - YAML: Dùng gopkg.in/yaml.v3 (lightweight)

## Migration Path

### Step 1: Add YAML Support (Non-Breaking)
- Thêm YAML loader
- Giữ hardcoded rules
- Engine dùng cả hai sources

### Step 2: Convert Rules Gradually
- Convert 1-2 rules sang YAML
- Test và validate
- Migrate từng rule một

### Step 3: Make YAML Primary
- YAML rules là primary
- Hardcoded rules là fallback
- Remove hardcoded khi đã migrate hết

## File Structure

```
core/pkg/riskengine/
├── engine.go              # Enhanced engine
├── rule.go                # Rule structs (extended)
├── insight_manager.go     # Existing
├── loader/                # NEW
│   ├── yaml_loader.go     # YAML rule loader
│   ├── validator.go       # Simple validation
│   └── compiler.go        # Expression compiler
└── registry/              # NEW
    ├── rule_registry.go   # Rule registry
    └── cache.go           # LRU cache

core/rules/                # NEW
├── cis-5.1.3.yaml
├── wildcard-permissions.yaml
└── ...
```

## Resource Limits

- **Memory per rule**: < 10KB (uncompiled), < 50KB (compiled)
- **Total rules**: Support up to 100 rules
- **Cache size**: Max 100 compiled expressions (LRU)
- **Path cache**: Max 1000 paths
- **Load time**: < 100ms for 20 rules
- **Evaluation time**: < 5ms per resource

## Testing Strategy

1. **Unit Tests**: Test loader, validator, compiler
2. **Integration Tests**: Test với existing engine
3. **Performance Tests**: Benchmark memory và CPU
4. **Backward Compatibility**: Đảm bảo existing rules vẫn work

## Next Steps

1. ✅ Review và approve plan
2. ✅ Implement Phase 1 (YAML Loader)
3. ✅ Test với 1-2 rules
4. ✅ Measure resource usage
5. ✅ Iterate và optimize

