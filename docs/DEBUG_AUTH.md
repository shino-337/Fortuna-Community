# Debug Authentication Issues

Hướng dẫn debug các vấn đề authentication.

## Debug Steps

### 1. Check Browser Console

Mở browser console (F12) và check logs:
- `[AuthContext]` logs - Authentication state
- `[ProtectedRoute]` logs - Route protection
- Network tab - API requests và responses

### 2. Check localStorage

```javascript
// In browser console
localStorage.getItem('ksam_token')
```

### 3. Check API Response

```javascript
// Test login API
fetch('http://localhost:8080/api/v1/auth/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ username: 'admin', password: 'admin123' })
})
.then(r => r.json())
.then(console.log)
```

### 4. Check Token in Requests

```javascript
// Check if token is in request headers
// Open Network tab, check Authorization header
```

## Common Issues

### Issue: Not redirecting to /login

**Check**:
1. ProtectedRoute có được render không?
2. `isAuthenticated` có đúng không?
3. `loading` có đúng không?

**Debug**:
```javascript
// In browser console
// Check AuthContext state
```

### Issue: Login fails

**Check**:
1. API endpoint có đúng không? (`/api/v1/auth/login`)
2. Request format có đúng không?
3. Response format có đúng không?

**Debug**:
```javascript
// Test login manually
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

### Issue: Token not added to requests

**Check**:
1. Request interceptor có hoạt động không?
2. Token có trong localStorage không?
3. Token format có đúng không?

**Debug**:
```javascript
// Check localStorage
localStorage.getItem('ksam_token')

// Check axios defaults
// In browser console after login
```

## Quick Fixes

### Clear localStorage và retry

```javascript
localStorage.removeItem('ksam_token')
location.reload()
```

### Manual login test

```bash
# Port-forward Core
kubectl port-forward -n ksam svc/ksam-core 8080:8080

# Test login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

### Check Core logs

```bash
kubectl logs -l app=ksam-core -n ksam --tail=50
```

