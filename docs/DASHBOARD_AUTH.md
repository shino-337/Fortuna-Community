# Dashboard Authentication Guide

Hướng dẫn về authentication trong Dashboard.

## Authentication Flow

1. **User truy cập Dashboard** → Redirect đến `/login` nếu chưa authenticated
2. **User login** → Token được lưu trong localStorage
3. **Token được tự động thêm vào mọi API request** qua request interceptor
4. **Nếu token expired (401)** → Tự động redirect về `/login`

## Components

### AuthContext (`src/contexts/AuthContext.tsx`)
- Quản lý authentication state
- Login/logout functions
- Token storage trong localStorage
- Auto-load token khi reload page

### Login Page (`src/pages/Login.tsx`)
- Login form với username/password
- Error handling
- Loading state
- Redirect sau khi login thành công

### ProtectedRoute (`src/components/ProtectedRoute.tsx`)
- Bảo vệ routes cần authentication
- Redirect to login nếu chưa authenticated
- Loading state

### API Interceptors (`src/services/api.ts`)
- **Request Interceptor**: Tự động thêm token vào mọi request
- **Response Interceptor**: Handle 401 errors và redirect to login

## API Endpoints

### Public
- `POST /api/v1/auth/login` - Login
- `POST /api/v1/auth/register` - Register (optional)

### Protected (require token)
- `GET /api/v1/auth/me` - Get current user
- `POST /api/v1/auth/change-password` - Change password
- Tất cả các endpoints khác

## Default Credentials

- **Username**: `admin`
- **Password**: `admin123`

## Token Storage

Token được lưu trong `localStorage` với key `ksam_token`.

## Troubleshooting

### Lỗi "Authorization header required"

**Nguyên nhân**: Token chưa được thêm vào request header.

**Giải pháp**:
1. Kiểm tra token có trong localStorage:
```javascript
localStorage.getItem('ksam_token')
```

2. Kiểm tra request interceptor:
```javascript
// Trong api.ts, request interceptor phải thêm token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('ksam_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})
```

3. Clear localStorage và login lại:
```javascript
localStorage.removeItem('ksam_token')
// Reload page và login lại
```

### Không redirect đến /login

**Nguyên nhân**: ProtectedRoute hoặc routing không hoạt động đúng.

**Giải pháp**:
1. Kiểm tra routing trong App.tsx
2. Kiểm tra ProtectedRoute component
3. Kiểm tra AuthContext có load đúng không

### Token expired nhưng không redirect

**Nguyên nhân**: Response interceptor không handle 401.

**Giải pháp**:
1. Kiểm tra response interceptor trong api.ts
2. Đảm bảo có handle 401 error

## Testing

### Test Login
```bash
# Port-forward Core
kubectl port-forward -n ksam svc/ksam-core 8080:8080

# Test login API
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

### Test Protected Endpoint
```bash
# Get token từ login response
TOKEN="your-token-here"

# Test protected endpoint
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/clusters
```

## Best Practices

1. **Token Storage**: Sử dụng localStorage (hoặc httpOnly cookies cho production)
2. **Token Expiration**: Check token expiration và refresh nếu cần
3. **Error Handling**: Handle 401 errors và redirect to login
4. **Loading States**: Show loading state khi check authentication
5. **Security**: Không log token trong console

