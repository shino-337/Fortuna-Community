# KSAM Security Guide

**Last Updated**: 2025-12-08  
**Version**: 2.0  
**Status**: Production Security Standards  
**Compliance**: MANDATORY for all developers

---

## Table of Contents

1. [Authentication & Authorization](#authentication--authorization)
2. [SQL Security](#sql-security) 🔒 **NEW**
3. [Input Validation](#input-validation) 🔒 **NEW**
4. [API Security](#api-security)
5. [Rate Limiting & DoS Protection](#rate-limiting--dos-protection) 🔒 **NEW**
6. [Security Headers](#security-headers)
7. [Secrets Management](#secrets-management) 🔒 **NEW**
8. [Audit Logging](#audit-logging)
9. [Security Testing](#security-testing) 🔒 **NEW**
10. [Incident Response](#incident-response) 🔒 **NEW**
11. [Troubleshooting](#troubleshooting)

---

## Authentication & Authorization

### Overview

KSAM Core uses JWT (JSON Web Tokens) for authentication with role-based access control (RBAC).

### Login

**Endpoint**: `POST /api/v1/auth/login`

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "password"
  }'
```

**Response**:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "username": "admin",
    "email": "admin@example.com",
    "role": "admin"
  },
  "expiresAt": "2024-01-01T00:00:00Z"
}
```

### Using the Token

Include the token in the Authorization header:

```bash
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/insights
```

### Register New User (Admin Only)

**Endpoint**: `POST /api/v1/auth/register`

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "newuser",
    "email": "user@example.com",
    "password": "securepassword",
    "role": "user"
  }'
```

### User Roles

| Role | Permissions | Use Case |
|------|-------------|----------|
| **admin** | Full access, user management, all CRUD operations | System administrators |
| **user** | View and edit resources, cannot create users | Security analysts |
| **viewer** | Read-only access | Auditors, managers |

### Password Requirements

**Enforced Policy**:
- Minimum length: 12 characters
- Must contain: uppercase, lowercase, number, special character
- Cannot be common password (e.g., "password123")
- Cannot be same as username
- Password history: Last 5 passwords remembered

**Implementation**:
```go
// File: pkg/auth/password.go

func ValidatePassword(password string, username string) error {
    if len(password) < 12 {
        return errors.New("password must be at least 12 characters")
    }
    
    if password == username {
        return errors.New("password cannot be same as username")
    }
    
    // Check complexity
    hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
    hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
    hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
    hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password)
    
    if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
        return errors.New("password must contain uppercase, lowercase, number, and special character")
    }
    
    return nil
}
```

### JWT Configuration

**Settings**:
- Algorithm: HS256
- Token expiration: 24 hours (configurable)
- Refresh token: 7 days (configurable)
- Secret key: `JWT_SECRET` environment variable

**Token Structure**:
```json
{
  "sub": "user_id",
  "username": "admin",
  "role": "admin",
  "iat": 1234567890,
  "exp": 1234654290
}
```

### Multi-Factor Authentication (MFA)

**Status**: Planned for MVP2  
**Methods**: TOTP (Google Authenticator, Authy)

---

## SQL Security

### 🚨 CRITICAL: SQL Injection Prevention

**Golden Rule**: NEVER trust user input in SQL queries.

### Safe SQL Patterns

#### ✅ Pattern 1: GORM Query Builder (Recommended)

```go
// ✅ SAFE: Automatic parameter binding
func GetInsights(db *gorm.DB, clusterID string, severity string) ([]models.Insight, error) {
    var insights []models.Insight
    
    err := db.Where("cluster_id = ?", clusterID).
        Where("severity = ?", severity).
        Where("deleted_at IS NULL").
        Find(&insights).Error
    
    return insights, err
}

// GORM generates: SELECT * FROM insights WHERE cluster_id = $1 AND severity = $2 AND deleted_at IS NULL
// Parameters: [clusterID, severity]
```

#### ✅ Pattern 2: Raw Query with Parameters

```go
// ✅ SAFE: Use $1, $2, $3... placeholders
func GetRiskTrends(db *gorm.DB, clusterID string, days int) ([]TrendData, error) {
    var trends []TrendData
    
    query := `
        SELECT 
            DATE(calculated_at) as date,
            AVG(total_score) as avg_score
        FROM risk_scores
        WHERE cluster_id = $1
            AND deleted_at IS NULL
            AND calculated_at >= NOW() - (INTERVAL '1 day' * $2)
        GROUP BY DATE(calculated_at)
        ORDER BY date ASC
    `
    
    err := db.Raw(query, clusterID, days).Scan(&trends).Error
    return trends, err
}
```

#### ✅ Pattern 3: Whitelist for Identifiers

```go
// ✅ SAFE: Validate against whitelist
func GetSortedData(db *gorm.DB, sortBy string) ([]models.Insight, error) {
    // Whitelist allowed columns
    allowedColumns := map[string]bool{
        "created_at":  true,
        "updated_at":  true,
        "severity":    true,
        "total_score": true,
    }
    
    if !allowedColumns[sortBy] {
        return nil, errors.New("invalid sort column")
    }
    
    var insights []models.Insight
    err := db.Order(sortBy + " DESC").Find(&insights).Error
    return insights, err
}
```

### ❌ Dangerous Patterns (NEVER DO!)

```go
// ❌ EXTREMELY DANGEROUS - SQL Injection!
func BAD_GetInsights(db *gorm.DB, clusterID string) error {
    // WRONG: String concatenation
    query := "SELECT * FROM insights WHERE cluster_id = '" + clusterID + "'"
    db.Raw(query).Scan(&results)
    
    // Attack: clusterID = "x' OR '1'='1' --"
    // Result: Returns ALL insights! 💀
}

// ❌ DANGEROUS - fmt.Sprintf with values
func BAD_GetRiskScores(db *gorm.DB, minScore float64) error {
    query := fmt.Sprintf("SELECT * FROM risk_scores WHERE total_score > %f", minScore)
    db.Raw(query).Scan(&scores)
    
    // Vulnerable to injection! 💀
}
```

### SQL Security Checklist

**Before merging any code with SQL**:

```
[ ] All user input uses parameter binding ($1, $2, $3...)
[ ] No string concatenation for SQL values
[ ] No fmt.Sprintf for SQL values
[ ] Table/column names validated against whitelist
[ ] Input validation before SQL execution
[ ] Query timeouts implemented (>2s)
[ ] Result limits enforced (<1000 rows)
[ ] Error messages don't leak SQL structure
[ ] SQL injection tests included
[ ] Code reviewed by security team
```

### Testing for SQL Injection

```go
// File: internal/api/insights_test.go

func TestSQLInjectionProtection(t *testing.T) {
    maliciousInputs := []string{
        "' OR '1'='1",
        "'; DROP TABLE insights; --",
        "x' UNION SELECT * FROM users --",
    }
    
    for _, input := range maliciousInputs {
        insights, err := GetInsightsByCluster(db, input)
        
        // Should return 0 results or error, NOT all data
        assert.True(t, err != nil || len(insights) == 0)
    }
}
```

**See Also**: [KSAM_SQL_SECURITY_GUIDELINES.md](./KSAM_SQL_SECURITY_GUIDELINES.md) for detailed SQL security documentation.

---

## Input Validation

### Validation Principles

**Trust Nothing**: All input is malicious until proven otherwise.

```
Validate:
├─ Query parameters (c.Query)
├─ Path parameters (c.Param)
├─ Request body (c.BindJSON)
├─ Headers (c.GetHeader)
├─ File uploads
└─ Webhook payloads
```

### Validation Patterns

#### Enum Validation

```go
func ValidateSeverity(severity string) error {
    validSeverities := []string{"low", "medium", "high", "critical"}
    
    for _, s := range validSeverities {
        if s == severity {
            return nil
        }
    }
    
    return errors.New("invalid severity: must be low, medium, high, or critical")
}

// Usage in handler
severity := c.Query("severity")
if err := ValidateSeverity(severity); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
}
```

#### Integer Range Validation

```go
func ValidateLimit(limitStr string, min, max int) (int, error) {
    limit, err := strconv.Atoi(limitStr)
    if err != nil {
        return 0, errors.New("limit must be an integer")
    }
    
    if limit < min || limit > max {
        return 0, fmt.Errorf("limit must be between %d and %d", min, max)
    }
    
    return limit, nil
}

// Usage
limit, err := ValidateLimit(c.Query("limit"), 1, 1000)
if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
}
```

#### Date Validation

```go
func ValidateDateRange(startDate, endDate string) (time.Time, time.Time, error) {
    start, err := time.Parse("2006-01-02", startDate)
    if err != nil {
        return time.Time{}, time.Time{}, errors.New("invalid start date format (YYYY-MM-DD)")
    }
    
    end, err := time.Parse("2006-01-02", endDate)
    if err != nil {
        return time.Time{}, time.Time{}, errors.New("invalid end date format (YYYY-MM-DD)")
    }
    
    if end.Before(start) {
        return time.Time{}, time.Time{}, errors.New("end date must be after start date")
    }
    
    // Prevent excessively large ranges
    if end.Sub(start) > 365*24*time.Hour {
        return time.Time{}, time.Time{}, errors.New("date range cannot exceed 1 year")
    }
    
    return start, end, nil
}
```

#### UUID Validation

```go
func ValidateUUID(uid string) error {
    // UUID format: 8-4-4-4-12 hex characters
    match := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(uid)
    
    if !match {
        return errors.New("invalid UUID format")
    }
    
    return nil
}
```

#### Kubernetes Resource Name Validation

```go
func ValidateK8sName(name string) error {
    // K8s naming rules: lowercase alphanumeric, hyphen, 1-253 chars
    if len(name) < 1 || len(name) > 253 {
        return errors.New("name must be 1-253 characters")
    }
    
    match := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`).MatchString(name)
    if !match {
        return errors.New("invalid Kubernetes resource name")
    }
    
    return nil
}
```

### Input Sanitization

**HTML Escaping**:
```go
import "html"

func SanitizeOutput(s string) string {
    return html.EscapeString(s)
}
```

**Path Traversal Prevention**:
```go
func ValidateFilePath(path string) error {
    // Prevent directory traversal
    if strings.Contains(path, "..") {
        return errors.New("invalid file path")
    }
    
    if strings.HasPrefix(path, "/") {
        return errors.New("absolute paths not allowed")
    }
    
    return nil
}
```

---

## API Security

### Protected Endpoints

All `/api/v1/*` endpoints require authentication except:
- `/health` - Health check
- `/metrics` - Prometheus metrics (should be internal only)
- `/api/v1/auth/login` - Login endpoint

### Role-Based Access Control (RBAC)

**Implementation**:
```go
// File: internal/middleware/rbac.go

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userRole := c.GetString("user_role")
        
        for _, role := range allowedRoles {
            if userRole == role {
                c.Next()
                return
            }
        }
        
        c.JSON(http.StatusForbidden, gin.H{
            "error": "insufficient permissions",
        })
        c.Abort()
    }
}

// Usage
router.DELETE("/api/v1/insights/:id", 
    middleware.RequireAuth(), 
    middleware.RequireRole("admin"),
    handlers.DeleteInsight)
```

**Permission Matrix**:

| Endpoint | Admin | User | Viewer |
|----------|-------|------|--------|
| GET /insights | ✅ | ✅ | ✅ |
| POST /insights | ✅ | ✅ | ❌ |
| PUT /insights/:id | ✅ | ✅ | ❌ |
| DELETE /insights/:id | ✅ | ❌ | ❌ |
| POST /users | ✅ | ❌ | ❌ |
| GET /audit-logs | ✅ | ❌ | ❌ |

### Request Size Limits

```go
// File: cmd/core/main.go

router.Use(func(c *gin.Context) {
    // Limit request body size to 10MB
    c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)
    c.Next()
})
```

### Response Filtering

**Never expose sensitive data**:

```go
// ❌ BAD: Exposes password hash
type User struct {
    ID           int    `json:"id"`
    Username     string `json:"username"`
    PasswordHash string `json:"password_hash"` // Leaked!
}

// ✅ GOOD: Separate response struct
type UserResponse struct {
    ID       int    `json:"id"`
    Username string `json:"username"`
    Email    string `json:"email"`
    Role     string `json:"role"`
    // No password hash
}

func GetUser(c *gin.Context) {
    var user models.User
    db.First(&user, c.Param("id"))
    
    // Return safe response
    c.JSON(200, UserResponse{
        ID:       user.ID,
        Username: user.Username,
        Email:    user.Email,
        Role:     user.Role,
    })
}
```

---

## Rate Limiting & DoS Protection

### Rate Limiting

**Global Rate Limit**: 100 requests per minute per IP

**Implementation**:
```go
// File: internal/middleware/ratelimit.go

import "golang.org/x/time/rate"

var limiters = make(map[string]*rate.Limiter)
var mu sync.Mutex

func RateLimitMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        
        mu.Lock()
        limiter, exists := limiters[ip]
        if !exists {
            // 100 requests per minute
            limiter = rate.NewLimiter(rate.Every(600*time.Millisecond), 100)
            limiters[ip] = limiter
        }
        mu.Unlock()
        
        if !limiter.Allow() {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error": "rate limit exceeded",
                "retry_after": 60,
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}
```

### Per-Endpoint Rate Limits

```go
// Expensive endpoints need stricter limits
router.GET("/api/v1/risk/trends", 
    middleware.RateLimitPerEndpoint("trends", 10, time.Minute), // 10/min
    handlers.GetRiskTrends)

router.GET("/api/v1/graph/attack-paths/:uid", 
    middleware.RateLimitPerEndpoint("graph", 5, time.Minute), // 5/min
    handlers.GetAttackPaths)
```

### Query Timeouts

**Prevent long-running queries**:

```go
func GetComplexData(db *gorm.DB) error {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    err := db.WithContext(ctx).Raw(query).Scan(&results).Error
    
    if errors.Is(err, context.DeadlineExceeded) {
        return errors.New("query timeout exceeded")
    }
    
    return err
}
```

### Result Set Limits

**Always limit query results**:

```go
func GetInsights(db *gorm.DB, limit int) ([]models.Insight, error) {
    // Enforce maximum limit
    maxLimit := 1000
    if limit > maxLimit || limit <= 0 {
        limit = maxLimit
    }
    
    var insights []models.Insight
    err := db.Limit(limit).Find(&insights).Error
    
    return insights, err
}
```

### Connection Pooling

```go
// File: pkg/storage/storage.go

func InitDB(dsn string) (*gorm.DB, error) {
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        return nil, err
    }
    
    sqlDB, err := db.DB()
    if err != nil {
        return nil, err
    }
    
    // Connection pool settings
    sqlDB.SetMaxIdleConns(10)           // Idle connections
    sqlDB.SetMaxOpenConns(100)          // Max connections
    sqlDB.SetConnMaxLifetime(time.Hour) // Connection lifetime
    
    return db, nil
}
```

---

## Security Headers

### Current Security Headers

The following security headers are automatically added to all responses:

```go
// File: internal/middleware/security.go

func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Prevent clickjacking
        c.Header("X-Frame-Options", "DENY")
        
        // Prevent MIME type sniffing
        c.Header("X-Content-Type-Options", "nosniff")
        
        // XSS protection (legacy, but doesn't hurt)
        c.Header("X-XSS-Protection", "1; mode=block")
        
        // HTTPS enforcement (production only)
        if os.Getenv("ENV") == "production" {
            c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        }
        
        // Content Security Policy
        c.Header("Content-Security-Policy", 
            "default-src 'self'; "+
            "script-src 'self' 'unsafe-inline'; "+
            "style-src 'self' 'unsafe-inline'; "+
            "img-src 'self' data: https:; "+
            "font-src 'self'; "+
            "connect-src 'self'; "+
            "frame-ancestors 'none'")
        
        // Referrer policy
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        
        // Permissions policy
        c.Header("Permissions-Policy", 
            "geolocation=(), "+
            "microphone=(), "+
            "camera=(), "+
            "payment=(), "+
            "usb=(), "+
            "magnetometer=()")
        
        c.Next()
    }
}
```

### CORS Configuration

```go
// File: internal/middleware/cors.go

func CORS() gin.HandlerFunc {
    return cors.New(cors.Config{
        AllowOrigins: []string{
            "http://localhost:3000",  // Dashboard (dev)
            "https://dashboard.ksam.io", // Dashboard (prod)
        },
        AllowMethods: []string{
            "GET", "POST", "PUT", "DELETE", "OPTIONS",
        },
        AllowHeaders: []string{
            "Origin", "Content-Type", "Authorization",
        },
        ExposeHeaders: []string{
            "Content-Length",
        },
        AllowCredentials: true,
        MaxAge:           12 * time.Hour,
    })
}
```

---

## Secrets Management

### Environment Variables

**Required (Production)**:
```bash
# JWT
JWT_SECRET="<random-256-bit-key>"

# Database
DB_PASSWORD="<strong-password>"

# Encryption
ENCRYPTION_KEY="<random-256-bit-key>"

# External Services (if applicable)
SLACK_WEBHOOK_URL="<webhook-url>"
EMAIL_PASSWORD="<app-password>"
```

### Secrets Best Practices

1. **Never commit secrets to Git**:
```bash
# .gitignore
.env
.env.local
*.key
*.pem
secrets/
```

2. **Use secrets management tools**:
```bash
# Kubernetes Secrets
kubectl create secret generic ksam-secrets \
  --from-literal=jwt-secret=<value> \
  --from-literal=db-password=<value>

# Deployment
env:
  - name: JWT_SECRET
    valueFrom:
      secretKeyRef:
        name: ksam-secrets
        key: jwt-secret
```

3. **Rotate secrets regularly**:
```bash
# JWT secret rotation
# 1. Generate new secret
NEW_SECRET=$(openssl rand -base64 32)

# 2. Update K8s secret
kubectl patch secret ksam-secrets -p '{"data":{"jwt-secret":"'$(echo -n $NEW_SECRET | base64)'"}}'

# 3. Rolling restart
kubectl rollout restart deployment/ksam-core
```

4. **Encrypt sensitive data at rest**:
```go
// File: pkg/crypto/encryption.go

import "crypto/aes"
import "crypto/cipher"

func Encrypt(plaintext []byte, key []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    
    return gcm.Seal(nonce, nonce, plaintext, nil), nil
}
```

### Password Storage

**NEVER store plaintext passwords**:

```go
// ✅ CORRECT: Hash with bcrypt
import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
    // Cost factor 12 (2^12 = 4096 iterations)
    hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
    return string(hash), err
}

func CheckPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}

// ❌ WRONG: Plaintext or weak hashing
password := "password123" // Never do this!
hash := md5.Sum([]byte(password)) // MD5 is broken!
```

---

## Audit Logging

### What to Log

**Security Events**:
```
✅ Authentication attempts (success/failure)
✅ Authorization failures
✅ Password changes
✅ User creation/deletion
✅ Permission changes
✅ Configuration changes
✅ Data access (sensitive resources)
✅ Data modifications
✅ API rate limit hits
✅ Suspicious activity
```

**Do NOT Log**:
```
❌ Passwords (even encrypted)
❌ JWT tokens
❌ Credit card numbers
❌ Social security numbers
❌ API keys/secrets
```

### Audit Log Schema

```sql
CREATE TABLE audit_logs (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    user_id INTEGER REFERENCES users(id),
    username VARCHAR(255),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id VARCHAR(255),
    ip_address INET,
    user_agent TEXT,
    request_method VARCHAR(10),
    request_path TEXT,
    status_code INTEGER,
    error_message TEXT,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
```

### Audit Logging Implementation

```go
// File: pkg/audit/logger.go

type AuditLogger struct {
    db *gorm.DB
}

func (a *AuditLogger) Log(ctx context.Context, event AuditEvent) error {
    log := AuditLog{
        UserID:        event.UserID,
        Username:      event.Username,
        Action:        event.Action,
        ResourceType:  event.ResourceType,
        ResourceID:    event.ResourceID,
        IPAddress:     event.IPAddress,
        UserAgent:     event.UserAgent,
        RequestMethod: event.RequestMethod,
        RequestPath:   event.RequestPath,
        StatusCode:    event.StatusCode,
        ErrorMessage:  event.ErrorMessage,
        Metadata:      event.Metadata,
    }
    
    return a.db.Create(&log).Error
}

// Middleware for automatic logging
func AuditMiddleware(auditLogger *AuditLogger) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        c.Next()
        
        // Log after request completes
        event := AuditEvent{
            UserID:        c.GetInt("user_id"),
            Username:      c.GetString("username"),
            Action:        c.Request.Method + " " + c.Request.URL.Path,
            IPAddress:     c.ClientIP(),
            UserAgent:     c.Request.UserAgent(),
            RequestMethod: c.Request.Method,
            RequestPath:   c.Request.URL.Path,
            StatusCode:    c.Writer.Status(),
            Metadata: map[string]interface{}{
                "duration_ms": time.Since(start).Milliseconds(),
                "query":       c.Request.URL.RawQuery,
            },
        }
        
        auditLogger.Log(c.Request.Context(), event)
    }
}
```

### Audit Log Retention

```sql
-- Delete logs older than 1 year
DELETE FROM audit_logs 
WHERE timestamp < NOW() - INTERVAL '1 year';

-- Or archive to cold storage
INSERT INTO audit_logs_archive 
SELECT * FROM audit_logs 
WHERE timestamp < NOW() - INTERVAL '90 days';

DELETE FROM audit_logs 
WHERE timestamp < NOW() - INTERVAL '90 days';
```

---

## Security Testing

### 1. SQL Injection Testing

```bash
# Install sqlmap
pip install sqlmap

# Test endpoint
sqlmap -u "http://localhost:8080/api/v1/insights?cluster_id=test" \
  --cookie="token=<jwt-token>" \
  --batch --level=5 --risk=3

# Expected: No vulnerabilities found
```

### 2. Authentication Testing

```go
// File: internal/api/auth_test.go

func TestAuthenticationRequired(t *testing.T) {
    router := setupRouter()
    
    // Test without token
    req := httptest.NewRequest("GET", "/api/v1/insights", nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    assert.Equal(t, 401, w.Code)
    assert.Contains(t, w.Body.String(), "authentication required")
}

func TestInvalidToken(t *testing.T) {
    router := setupRouter()
    
    req := httptest.NewRequest("GET", "/api/v1/insights", nil)
    req.Header.Set("Authorization", "Bearer invalid-token")
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    assert.Equal(t, 401, w.Code)
}
```

### 3. Authorization Testing

```go
func TestRBACEnforcement(t *testing.T) {
    // Viewer should not be able to delete
    token := generateToken("viewer", "viewer")
    
    req := httptest.NewRequest("DELETE", "/api/v1/insights/123", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    assert.Equal(t, 403, w.Code)
    assert.Contains(t, w.Body.String(), "insufficient permissions")
}
```

### 4. Input Validation Testing

```go
func TestInputValidation(t *testing.T) {
    testCases := []struct {
        input    string
        expected int
    }{
        {"' OR '1'='1", 400},      // SQL injection attempt
        {"<script>alert(1)</script>", 400}, // XSS attempt
        {"../../../../etc/passwd", 400},     // Path traversal
    }
    
    for _, tc := range testCases {
        req := httptest.NewRequest("GET", "/api/v1/insights?search="+tc.input, nil)
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        
        assert.Equal(t, tc.expected, w.Code)
    }
}
```

### 5. Rate Limiting Testing

```go
func TestRateLimit(t *testing.T) {
    router := setupRouter()
    
    // Make 101 requests (limit is 100)
    for i := 0; i < 101; i++ {
        req := httptest.NewRequest("GET", "/api/v1/insights", nil)
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        
        if i < 100 {
            assert.Equal(t, 200, w.Code)
        } else {
            assert.Equal(t, 429, w.Code) // Too Many Requests
        }
    }
}
```

### 6. Security Scanning

```bash
# Install gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Run security scan
gosec -fmt=json -out=gosec-report.json ./...

# Check for high/medium issues
cat gosec-report.json | jq '.Issues[] | select(.severity=="HIGH" or .severity=="MEDIUM")'
```

### 7. Dependency Scanning

```bash
# Check for vulnerable dependencies
go list -json -m all | nancy sleuth

# Or use govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

---

## Incident Response

### Security Incident Classification

| Severity | Description | Response Time | Examples |
|----------|-------------|---------------|----------|
| **P0 - Critical** | Active breach, data loss | Immediate | SQL injection exploit, data exfiltration |
| **P1 - High** | Potential breach, system compromise | 1 hour | Authentication bypass, privilege escalation |
| **P2 - Medium** | Security vulnerability discovered | 24 hours | Unpatched CVE, weak configuration |
| **P3 - Low** | Security improvement needed | 1 week | Missing security header, weak password policy |

### Incident Response Playbook

**1. Detection**:
```
✅ Monitor logs for suspicious activity
✅ Set up alerts for security events
✅ Regular security audits
✅ Bug bounty reports
```

**2. Containment**:
```
✅ Isolate affected systems
✅ Revoke compromised credentials
✅ Block malicious IPs
✅ Take offline if necessary
```

**3. Eradication**:
```
✅ Patch vulnerabilities
✅ Remove malicious code
✅ Reset passwords
✅ Rotate secrets
```

**4. Recovery**:
```
✅ Restore from backups
✅ Verify system integrity
✅ Gradual service restoration
✅ Monitor for reinfection
```

**5. Post-Incident**:
```
✅ Document incident
✅ Root cause analysis
✅ Update security measures
✅ Team training
✅ Notify affected parties (if required)
```

### Emergency Contacts

```
Security Team: security@company.com
On-Call: +1-XXX-XXX-XXXX
Slack: #security-incidents
PagerDuty: security-oncall
```

### Security Event Monitoring

**Set up alerts for**:
```sql
-- Failed login attempts (>5 in 1 minute)
SELECT user_id, COUNT(*) as attempts
FROM audit_logs
WHERE action = 'LOGIN_FAILED'
  AND timestamp > NOW() - INTERVAL '1 minute'
GROUP BY user_id
HAVING COUNT(*) > 5;

-- Unauthorized access attempts
SELECT user_id, COUNT(*) as attempts
FROM audit_logs
WHERE status_code = 403
  AND timestamp > NOW() - INTERVAL '5 minutes'
GROUP BY user_id
HAVING COUNT(*) > 10;

-- SQL injection attempts (suspicious query patterns)
SELECT ip_address, COUNT(*) as attempts
FROM audit_logs
WHERE request_path LIKE '%''%'
   OR request_path LIKE '%OR%'
   OR request_path LIKE '%DROP%'
GROUP BY ip_address;
```

---

## Environment Variables

### Required (Production)

```bash
# JWT Authentication
JWT_SECRET="<random-256-bit-key>"           # CRITICAL: Change in production!
TOKEN_EXPIRATION_HOURS=24

# Database
DB_HOST="localhost"
DB_PORT="5432"
DB_USER="ksam"
DB_PASSWORD="<strong-password>"            # CRITICAL: Use strong password
DB_NAME="ksam_db"
DB_SSL_MODE="require"                      # CRITICAL: Use SSL in production

# Admin User (Initial Setup)
KSAM_ADMIN_USERNAME="admin"
KSAM_ADMIN_PASSWORD="<strong-password>"    # CRITICAL: Change immediately
KSAM_ADMIN_EMAIL="admin@company.com"

# Security
AUTH_ENABLED=true                          # Never disable in production
ENCRYPTION_KEY="<random-256-bit-key>"      # For data encryption at rest

# Rate Limiting
RATE_LIMIT_REQUESTS=100                    # Requests per minute
RATE_LIMIT_BURST=200                       # Burst capacity

# Logging
LOG_LEVEL="info"                           # info, warn, error
AUDIT_LOG_RETENTION_DAYS=365
```

### Optional

```bash
# Features
ENABLE_MFA=false                           # Coming in MVP2
ENABLE_PASSWORDLESS=false                  # Future feature

# External Services
SLACK_WEBHOOK_URL=""                       # For notifications
EMAIL_SMTP_HOST=""
EMAIL_SMTP_PORT="587"
EMAIL_SMTP_USERNAME=""
EMAIL_SMTP_PASSWORD=""

# Monitoring
SENTRY_DSN=""                              # Error tracking
PROMETHEUS_ENABLED=true
```

### Generating Secure Secrets

```bash
# JWT Secret (256-bit)
openssl rand -base64 32

# Encryption Key (256-bit)
openssl rand -base64 32

# Database Password (strong)
openssl rand -base64 24
```

---

## Best Practices

### Development

1. **Never commit secrets**: Use `.env` files and `.gitignore`
2. **Use parameter binding**: Always use `$1, $2` in SQL queries
3. **Validate input**: Check all user input before processing
4. **Test security**: Write tests for SQL injection, XSS, etc.
5. **Code review**: All PRs must be reviewed for security
6. **Update dependencies**: Regular `go get -u` for security patches

### Deployment

1. **Change all secrets**: Generate new secrets for each environment
2. **Use HTTPS only**: Force HTTPS in production
3. **Enable audit logging**: Full audit trail required
4. **Set up monitoring**: Alerts for security events
5. **Regular backups**: Encrypted backups with tested restore
6. **Principle of least privilege**: Minimal permissions for services

### Operations

1. **Monitor logs**: Review audit logs daily
2. **Rotate secrets**: JWT secret every 90 days, DB password every 180 days
3. **Update systems**: Apply security patches within 7 days
4. **Security audits**: Quarterly penetration testing
5. **Incident drills**: Practice incident response procedures
6. **Access review**: Quarterly review of user permissions

---

## Troubleshooting

### Token Expired

**Symptom**: 401 Unauthorized error

**Solution**:
```bash
# Login again to get new token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'
```

### Invalid Token

**Symptom**: 401 Unauthorized with "invalid token"

**Check**:
1. Token format: `Authorization: Bearer <token>`
2. Token not tampered with
3. `JWT_SECRET` matches between services
4. Token not expired (check `exp` claim)

```bash
# Decode JWT to check claims (without verification)
echo "eyJhbGc..." | base64 -d | jq
```

### SQL Injection Alert

**Symptom**: Audit log shows suspicious SQL patterns

**Actions**:
1. Block offending IP immediately
2. Review affected queries
3. Check for data exfiltration
4. Patch vulnerable endpoint
5. Rotate credentials if compromised

### Rate Limit Hit

**Symptom**: 429 Too Many Requests

**Solution**:
```bash
# Wait for rate limit window to reset (1 minute)
sleep 60

# Or request rate limit increase (for legitimate use)
```

### User Inactive

**Symptom**: Login fails with valid credentials

**Check**:
```sql
SELECT id, username, is_active, last_login 
FROM users 
WHERE username = 'targetuser';
```

**Solution** (Admin):
```sql
UPDATE users SET is_active = true WHERE username = 'targetuser';
```

---

## Security Checklist

### Pre-Production

```
[ ] All default passwords changed
[ ] JWT_SECRET is random 256-bit value
[ ] Database uses SSL connection
[ ] HTTPS enabled and enforced
[ ] Security headers configured
[ ] Rate limiting enabled
[ ] Audit logging enabled
[ ] Secrets stored in secrets manager
[ ] SQL injection testing passed
[ ] Authentication testing passed
[ ] RBAC testing passed
[ ] Dependency scan clean
[ ] Security scan (gosec) clean
[ ] Monitoring and alerts configured
[ ] Incident response plan documented
[ ] Team trained on security procedures
```

### Post-Production

```
[ ] Monitor audit logs daily
[ ] Review security alerts
[ ] Apply security patches weekly
[ ] Rotate secrets quarterly
[ ] Security audit annually
[ ] Penetration test annually
[ ] Update documentation
[ ] Team security training quarterly
```

---

## Resources

### Internal Documentation

- [SQL Security Guidelines](./KSAM_SQL_SECURITY_GUIDELINES.md)
- [API Documentation](./API.md)
- [Architecture Overview](./ARCHITECTURE.md)

### External Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CWE Top 25](https://cwe.mitre.org/top25/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
- [Go Security Best Practices](https://golang.org/doc/security)

### Tools

- [gosec](https://github.com/securego/gosec) - Go security scanner
- [sqlmap](https://sqlmap.org/) - SQL injection testing
- [nancy](https://github.com/sonatype-nexus-community/nancy) - Dependency checker
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) - Go vulnerability checker

---

**Last Updated**: 2025-12-08  
**Version**: 2.0  
**Maintained By**: Security Team

**For security issues**: security@company.com  
**For questions**: Slack #security
