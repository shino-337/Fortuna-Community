# 🚀 Landing Page Access Guide

## Quick Start

### Option 1: Auto-Launch Script (Recommended)

```bash
cd /Users/tuatnh/Desktop/Learn/K8s\ Service\ Account\ Management\ Platform/KSAM
./scripts/open-landing.sh
```

This script will:
- ✅ Kill existing port-forwards on port 3000
- ✅ Start new port-forward: `localhost:3000 → ksam-dashboard:80`
- ✅ Open browser to landing page
- ✅ Display all access URLs

### Option 2: Manual Port-Forward

```bash
kubectl port-forward -n ksam svc/ksam-dashboard 3000:80
```

Then open: **http://localhost:3000**

### Option 3: Direct NodePort Access

```bash
# Get Minikube IP and NodePort
minikube ip  # Returns: 192.168.49.2
kubectl get svc -n ksam ksam-dashboard  # NodePort: 30080
```

Open: **http://192.168.49.2:30080**

---

## 🔥 Important: Browser Cache Issue

### Why You Might See Login Page Instead of Landing Page

If you've previously logged in, your browser has cached an **auth token** in localStorage. The app detects this token and automatically redirects you to the dashboard.

### Solutions

#### 1️⃣ Use Incognito/Private Window (Fastest)

**Chrome/Edge:**
```
Cmd + Shift + N  (Mac)
Ctrl + Shift + N  (Windows)
```

**Firefox:**
```
Cmd + Shift + P  (Mac)
Ctrl + Shift + P  (Windows)
```

**Safari:**
```
Cmd + Shift + N
```

Then navigate to: `http://localhost:3000`

#### 2️⃣ Clear Browser Storage

1. Open DevTools: `F12` or `Cmd+Option+I` (Mac)
2. Go to **Application** tab (Chrome) or **Storage** tab (Firefox)
3. Click **"Clear site data"** or **"Clear All"**
4. Reload page: `Cmd+R` or `Ctrl+R`

#### 3️⃣ Use Test Tool

Open the test tool:
```bash
open /Users/tuatnh/Desktop/Learn/K8s\ Service\ Account\ Management\ Platform/KSAM/test-landing.html
```

Click the **"Clear Auth & Reload"** button.

---

## 📊 Port Reference

| Port | Service | Access URL | Status |
|------|---------|------------|--------|
| 3000 | Port-Forward (Local) | http://localhost:3000 | ✅ Recommended |
| 30080 | NodePort (Minikube) | http://192.168.49.2:30080 | ✅ Direct |
| 80 | Service Internal | N/A | Internal only |

---

## 🎯 What You Should See

### Landing Page Components:

1. **Navigation Bar**
   - Logo: K8sWorkload
   - Links: Platform, Features, Docs
   - Button: LOGIN

2. **Hero Section**
   - Large Helm logo (animated, pink gradient)
   - Title: "K8s Workload Management Platform"
   - Tagline: "Visibility First. Security Always."
   - CTA Buttons:
     - "Access Dashboard" (pink) → /login
     - "View on GitHub" (outline) → GitHub link

3. **Features Section**
   - Deep Visibility
   - RBAC Management
   - Audit Logging
   - Multi-Cluster

4. **Call-to-Action Banner**
   - "Start Managing Your Workloads"
   - "Get Started" button → /login

5. **Footer**
   - Copyright
   - Social links (GitHub, Twitter, LinkedIn)
   - Privacy, Terms, Contact links

---

## 🔍 Troubleshooting

### Port 3000 Already in Use

```bash
# Find process using port 3000
lsof -i :3000

# Kill the process
lsof -ti:3000 | xargs kill -9

# Restart port-forward
./scripts/open-landing.sh
```

### Port-Forward Disconnects

Port-forward may disconnect after a while. Simply re-run:

```bash
./scripts/open-landing.sh
```

### Still See Login Page After Clearing Cache

1. **Check if token is really cleared:**
   - Open DevTools → Console
   - Type: `localStorage.getItem('token')`
   - Should return: `null`

2. **If token still exists:**
   ```javascript
   localStorage.clear()
   sessionStorage.clear()
   location.reload()
   ```

3. **Nuclear option (clears ALL site data):**
   - Chrome: Settings → Privacy → Clear browsing data
   - Select "Cookies and other site data"
   - Clear for "All time"

---

## 🧪 Testing Flow

### 1. Landing Page (Unauthenticated)
- Visit: http://localhost:3000
- Should see: Landing page with Hero + Features

### 2. Click "Access Dashboard"
- Should redirect to: http://localhost:3000/login
- Should see: Login form

### 3. Login
- Enter credentials
- Should redirect to: http://localhost:3000/dashboard
- Should see: Dashboard with navigation

### 4. Navigate Dashboard
- Click "Graph" → Graph View
- Click "ServiceAccounts" → ServiceAccounts page
- Click "Audit Logs" → Audit Logs page

### 5. Logout
- Click "Logout" button
- Should redirect to: http://localhost:3000/ (Landing page)

---

## 📝 Development Notes

### Build Version
- Current: **v4.0.2**
- Build hash: `index-eZHBBr2E.js`
- CSS hash: `index-DK66QXUe.css`

### Routing Logic
```typescript
// src/App.tsx
<Route 
  path="/" 
  element={
    isAuthenticated ? (
      <Navigate to="/dashboard" replace />
    ) : (
      <LandingPage />
    )
  } 
/>
```

### Auth Check
```typescript
// AuthContext checks localStorage for 'token'
const token = localStorage.getItem('token')
const isAuthenticated = !!token
```

---

## 🔗 Related Files

- Landing Page Component: `dashboard/src/pages/LandingPage.tsx`
- Routing: `dashboard/src/App.tsx`
- Auth Context: `dashboard/src/contexts/AuthContext.tsx`
- Launch Script: `scripts/open-landing.sh`
- Test Tool: `test-landing.html`

---

## 📸 Screenshots

See: `landing-page-success.png` for visual reference.

---

## 💡 Tips

1. **Always use Incognito** for testing landing page
2. **Use localhost:3000** (port-forward) instead of NodePort for faster access
3. **Run launch script** instead of manual port-forward (auto-cleanup)
4. **Check port-forward logs** if connection issues: `tail -f /tmp/ksam-port-forward.log`

---

**Last Updated**: 2025-11-21  
**Version**: v4.0.2

