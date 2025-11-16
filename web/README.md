# Flux Status - Frontend

SPA built with Preact + Tailwind + Vite.

## Development

Install dependencies:

```bash
make web-install
```

Start dev server with hot reload and mock data:

```bash
make web-dev-mock
```

To use a live backend connected to Kubernetes:

```bash
make web-run
make web-dev
```

Run unit tests:

```bash
make web-test
```

## Features

- **Dark/Light theme** - Auto mode theme to follow system preference
- **Responsive design** - Mobile-friendly layout
- **State preservation** - Expanded/collapsed sections persist during refresh & hot reload
- **Component-based architecture** - Using Preact Signals for reactive state
- **Long polling** - Auto-refresh every 30 seconds
- **Hot Module Replacement** - Instant updates during development

## Tech Stack

- **Preact**: Lightweight React alternative
- **Preact Signals**: Reactive state management
- **Tailwind CSS**: Utility-first CSS framework
- **Vite**: Fast build tool and dev server with HMR

## Mock Data

Mock data for UI development is organized in `src/mock/`:

- `report.js` - FluxReport mock data (`mockReport`)
- `events.js` - Events API mock data (`mockEvents`)
- `resources.js` - Resources API mock data (`mockResources`)

When `VITE_USE_MOCK_DATA=true`, the app uses dynamic imports to load mock data only when needed, preventing mock data from being bundled in production builds.

**Important**: Keep mock data in sync with the real API shape!

## Go Backend

- **Server setup**: `internal/web/serve.go` (HTTP server with graceful shutdown)
- **API routes**: `internal/web/router.go` registers HTTP handlers for `/api/v1/`
- **Embedded frontend**: Embedded `web/dist` via `//go:embed` in `web/embed.go` and serves it in `internal/web/fs.go`

## Component and Signal Naming Conventions

This project follows strict naming conventions for components and signals to maintain consistency and readability.

### Component Naming

Components use descriptive names with consistent suffixes:

- **List suffix**: Used for components that display collections of items
  - `ComponentList` - Displays Flux component controllers
  - `ReconcilerList` - Displays Flux reconciler types
  - `EventList` - Displays Kubernetes events
  - `ResourceList` - Displays Flux resource statuses

- **Status suffix**: Used for components that show status information
  - `ClusterStatus` - Overall cluster health
  - `ConnectionStatus` - Server connection state

- **Descriptive names**: Other components use clear, single-purpose names
  - `ClusterSync` - Cluster sync configuration and status
  - `ClusterInfo` - Cluster metadata and operator version
  - `SearchView` - Container for search functionality with tabs
  - `Header` - Application header with navigation
  - `Footer` - Application footer

### Signal Naming Conventions

Signals follow specific patterns based on their purpose:

#### A. Component Visibility Toggle (Show/Hide Content)

**Pattern**: `isExpanded`

**Usage**: Controls whether a component's main content is visible or collapsed. This signal is local (not exported) and scoped to the component file.

**Examples**:
```javascript
// ComponentList.jsx
const isExpanded = signal(true) // Show/hide components table

// ReconcilerList.jsx
const isExpanded = signal(true) // Show/hide reconcilers grid

// ClusterSync.jsx
const isExpanded = signal(true) // Show/hide sync details

// ClusterInfo.jsx
const isExpanded = signal(true) // Show/hide cluster info panel
```

**Why this pattern**: All these signals control the same behavior (toggle visibility of the component's content). The component file name provides context, so `isExpanded` is clear and consistent.

#### B. Multi-Item Expansion Tracking

**Pattern**: `expanded[ItemType]s`

**Usage**: Tracks which items in a collection have their details expanded. Uses a Set or array of identifiers.

**Example**:
```javascript
// ComponentList.jsx
const expandedComponentRows = signal(new Set()) // Tracks which rows are expanded
```

**Why this pattern**: Uses descriptive suffix (`Rows`, `Cards`, `Items`) to clarify what's being tracked, avoiding confusion with the component visibility toggle.

#### C. Data Fetch State (API Loading Pattern)

**Pattern**: `[feature]Data`, `[feature]Loading`, `[feature]Error`

**Usage**: Standard trio for async data fetching state management.

**Examples**:
```javascript
// EventList.jsx
export const eventsData = signal([])              // Array of events
export const eventsLoading = signal(false)        // Loading state
export const eventsError = signal(null)           // Error message or null

// ResourceList.jsx
export const resourcesData = signal([])           // Array of resources
export const resourcesLoading = signal(false)     // Loading state
export const resourcesError = signal(null)        // Error message or null
```

**Why this pattern**: Consistent three-signal pattern makes data fetching predictable across the codebase.

#### D. Filter State (User Input Filters)

**Pattern**: `selected[Feature][FilterType]`

**Usage**: User-controlled filter values for search/filter functionality.

**Examples**:
```javascript
// EventList.jsx
export const selectedEventsKind = signal('')      // Filter by resource kind
export const selectedEventsName = signal('')      // Filter by resource name
export const selectedEventsNamespace = signal('') // Filter by namespace

// ResourceList.jsx
export const selectedResourceKind = signal('')       // Filter by resource kind
export const selectedResourceName = signal('')       // Filter by resource name
export const selectedResourceNamespace = signal('')  // Filter by namespace
```

**Why this pattern**: Feature prefix prevents naming collisions when multiple components have similar filters.

#### E. View/Navigation State

**Pattern**: `show[ViewName]` or `active[ViewName]`

**Usage**: Controls which view or tab is displayed.

**Guidelines**:
- Use `show` prefix for boolean toggles (show/hide)
- Use `active` prefix for multi-option selection (tabs, radio groups)

**Examples**:
```javascript
// app.jsx
export const showSearchView = signal(false) // Boolean: dashboard or search

// SearchView.jsx
export const activeSearchTab = signal('events') // Enum: 'events' | 'resources'
```

**Why this pattern**: Clear distinction between boolean toggles (`show`) and multi-value selection (`active`).

#### F. Component-Level useState (Card/Row Expansion)

**Pattern**: `isExpanded` (local to the card/row component)

**Usage**: Local state for individual card or row message expansion (not a signal, uses useState).

**Examples**:
```javascript
// EventCard component in EventList.jsx
const [isExpanded, setIsExpanded] = useState(false) // Show full or truncated message

// ResourceCard component in ResourceList.jsx
const [isExpanded, setIsExpanded] = useState(false) // Show full or truncated message

// ComponentRow component in ComponentList.jsx
const [isRowExpanded, setIsRowExpanded] = useState(false) // Show component details
```

**Why this pattern**: Local component state doesn't need to be shared, so useState is more appropriate than signals. The naming is scoped to the component instance.

### Global Signals Reference

#### Application State (app.jsx)
```javascript
fluxReport          // Main FluxReport data (null | object)
lastUpdated         // Timestamp of last fetch (Date)
isLoading           // Global loading state (boolean)
connectionStatus    // 'loading' | 'connected' | 'disconnected'
showSearchView      // View toggle: false=dashboard, true=search
fetchFluxReport()   // Function to fetch report data
```

#### Theme State (theme.js)
```javascript
themeMode          // User selection: 'light' | 'dark' | 'auto'
appliedTheme       // Computed theme: 'light' | 'dark'
cycleTheme()       // Function to cycle through theme modes
```

### Naming Guidelines Summary

1. **Consistency**: Use the same name for the same purpose across files
2. **Scope awareness**: Local signals can use simple names (`isExpanded`); exported signals need prefixes (`selectedEventsKind`)
3. **Descriptive suffixes**: When tracking collections, use descriptive suffixes (`expandedComponentRows`, not just `expanded`)
4. **Pattern adherence**: Follow established patterns for common use cases (data fetching, filtering, navigation)
5. **Avoid overloading**: Don't use the same word (like "expanded") for different purposes in the same component
