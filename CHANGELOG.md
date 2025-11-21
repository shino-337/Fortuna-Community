# Changelog

All notable changes to the KSAM project will be documented in this file.

## [v4.3.0] - 2025-11-21
### Added
- **D3 SVG Graph Visualization**: Complete migration from Canvas to SVG rendering
  - Native D3.js force simulation with superior control and debugging
  - Clean SVG rendering with proper arrow markers on edges
  - Advanced collision detection using `d3.forceCollide()`
  - Multiple layout algorithms: Force-directed, Radial, Tree, Grid
  - Smooth zoom and pan with D3 zoom behavior
  - Node selection highlights with connected node emphasis
  - Interactive node dragging with physics simulation
  - Automatic graph fitting with proper padding
  - Pink theme colors matching K8s Fortuna branding

- **New Components**:
  - `constants.ts`: Centralized configuration (NODE_COLORS, NODE_RADIUS, FORCE_PARAMS, ZOOM_PARAMS)
  - `types.ts`: TypeScript types for RbacNode, RbacLink extending d3.SimulationNodeDatum
  - `ForceGraphSvg.tsx`: Pure D3+SVG graph renderer with advanced features
  - `GraphVisualizationD3.tsx`: Wrapper component integrating with existing KSAM API

### Changed
- **Graph Rendering Engine**: Migrated from react-force-graph-2d (Canvas) to D3.js (SVG)
  - Better performance for node selection and highlighting
  - Easier debugging with inspectable SVG DOM elements
  - More precise control over node/edge styling
  - Smoother animations and transitions
  
- **Node Colors**: Updated to pink theme for K8s Fortuna branding
  - Service Account: Pink 500 (#ec4899)
  - User: Pink 600 (#db2777)
  - Group: Pink 700 (#be185d)
  - Roles: Purple shades (500-600)
  - Bindings: Amber shades (500-600)
  - Infrastructure: Blue/Cyan shades
  
- **Layout Options**: Enhanced with D3 native layouts
  - Force-directed (default): Organic, physics-based
  - Radial: Circular distribution from center
  - Tree: Hierarchical top-down
  - Grid: Fixed grid positions
  
- **Visual Improvements**:
  - Arrow markers on all edges
  - Better node sizing by type (12-28px radius)
  - Improved label rendering with text shadows
  - Selection highlights with thick white stroke
  - Connected nodes show pink (#f472b6) outline
  - Dimmed unrelated nodes/edges on selection
  - Floating stats overlay (nodes/links/layout)

### Technical
- Added dependencies: `d3@^7.9.0`, `@types/d3@^7.4.3`
- Retained existing dependencies for backward compatibility
- Docker image: `ksam-dashboard:v4.3.0-d3svg`
- CSS bundle: 40.01 KB (unchanged)
- JS bundle: 402.09 KB (up from 373.89 KB due to full D3 library)

### Migration Notes
- Old `ForceGraphVisualization.tsx` (Canvas) retained for reference
- New `GraphVisualizationD3.tsx` now used in `GraphView.tsx`
- All existing filters and features preserved
- API data transformation handled transparently
- No breaking changes to user-facing features

## [v4.2.1] - 2025-11-21
### Changed
- **Login Page Redesign**: Complete redesign matching landing page aesthetic
  - Created reusable `HelmLogo` component for consistent branding
  - Black background with radial gradient texture
  - Pink gradient decorative top line
  - HelmLogo with glow effect and hover animation
  - "K8s Fortuna" branding with tagline
  - Modern input fields with icons (User, Lock)
  - Pink gradient "AUTHENTICATE" button with hover effects
  - "Back to Platform" link to return to landing page
  - Footer with "Protected by K8s Fortuna Identity Guard v2.4" text
  
- **Auth Logic**: Maintained all existing authentication functionality
  - Username/password authentication (admin/admin123)
  - Error handling with styled error messages
  - Loading states during authentication
  - Redirect to dashboard after successful login
  - Auto-redirect if already authenticated

### Technical
- Created `/src/components/HelmLogo.tsx` for reusable logo component
- Updated `/src/pages/Login.tsx` with new design while preserving auth logic
- Removed `/src/pages/Login_update.tsx` after integration
- Docker image: `ksam-dashboard:v4.2.1`
- CSS bundle: 38.99 KB

## [v4.2.0] - 2025-11-21
### Changed
- **Branding Update**: Unified brand identity across entire application
  - Updated favicon to HelmLogo SVG (pink gradient ship wheel)
  - Changed site title to "K8s Fortuna - Workload Management Platform"
  - Updated logo in Navigation bar with HelmLogo icon
  - Project name now displays as "K8s**Fortuna**" with pink accent
  
- **Color Scheme**: Harmonized colors with landing page theme
  - Navigation active state: `blue-600` → `pink-600`
  - Logo gradient: `blue-500/purple-600` → `pink-600/pink-800`
  - Consistent pink-600 accent color across all pages

### Technical
- Created `/public/icon.svg` with HelmLogo design
- Updated `index.html` meta description and title
- Modified `App.tsx` Navigation component with new logo and colors
- Docker image: `ksam-dashboard:v4.2.0`

## [v4.1.4] - 2025-11-21
### Fixed
- **Landing Page Scroll**: Fixed `overflow-hidden` issue that prevented scrolling
- **Root Layout**: Conditionally apply layout constraints only for authenticated users
- **User Experience**: Landing page now scrolls smoothly

### Technical
- Created `AppContent` wrapper component with authentication check
- Docker image: `ksam-dashboard:v4.1.4`

## [v4.1.3] - 2025-11-21
### Fixed
- **Complete cache clear and rebuild**: Verified all Tailwind classes correctly compiled

## [v4.1.2] - 2025-11-21
### Fixed
- **Product Branding**: Changed from "K8s Workload" to "K8s Fortuna"
- **Features Navigation**: Added `id="features"` for anchor links

## [v4.1.1] - 2025-11-21
### Changed
- **Single-Page Layout**: Adjusted Hero section to `min-h-[85vh]`

## [v4.1.0] - 2025-11-21
### Changed
- **Full Landing Page Content**: Complete feature descriptions

## [v4.0.2] - 2025-11-21
### Fixed
- **Port Forwarding**: Standardized on port 3000

## [v4.0.0] - 2025-11-21
### Added
- **Landing Page**: Initial integration

## [v3.3.0] - 2025-11-20
### Changed
- **Harmonized Spacing**: Standardized padding across Graph View

## [v3.2.0] - 2025-11-20
### Changed
- **Page Layout Standardization**: Consistent height management
- **Dark Mode Fixes**: Comprehensive dark mode improvements

## [v3.1.0] - 2025-11-20
### Changed
- **Top Bar Redesign**: CSS Grid layout for alignment
- **Root Layout Overhaul**: Consistent overflow management

## [v3.0.0] - 2025-11-20
### Changed
- **Force Graph Migration**: Migrated to `react-force-graph-2d`
- **3-Panel Layout**: New design with Filters, Canvas, Details

### Dependencies
- Added: `react-force-graph-2d`, `d3-force`, `d3-scale-chromatic`
- Removed: `cytoscape`, `react-cytoscapejs`, etc.
