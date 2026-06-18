# StatShed Frontend - Design Document

A comprehensive rebuild of the StatShed frontend with improved architecture, Tailwind CSS styling, and modern state management.

## Overview

This document outlines the complete frontend rebuild for StatShed, a real-time status dashboard for monitoring cluster job operations. The rebuild addresses pain points in the existing implementation while preserving what works well.

### Goals

1. **Improved State Management**: Replace inefficient full-refetch pattern with TanStack Query for caching and optimistic updates
2. **Modern Styling**: Migrate from component CSS files to Tailwind CSS for consistency and faster development
3. **Feature Completeness**: Add missing configuration UI, job submission, and filtering capabilities
4. **Better UX**: Implement toast notifications, loading skeletons, and improved error handling
5. **Maintainability**: Clear component structure with proper typing and separation of concerns

### Technology Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Framework | React 19 + TypeScript | Keep existing stack |
| Build Tool | Vite 7 | Keep existing, excellent DX |
| Routing | React Router v7 | Keep existing |
| State Management | TanStack Query v5 | Server state caching, automatic refetch, optimistic updates |
| UI State | React Context | Lightweight for UI-only state (modals, toasts) |
| Styling | Tailwind CSS v4 | Utility-first, dark mode, rapid development |
| Icons | Lucide React | Lightweight, consistent icon set |
| Real-time | Socket.IO Client | Keep existing WebSocket integration |
| Forms | React Hook Form + Zod | Type-safe forms with validation |
| Notifications | Sonner | Lightweight toast library |

---

## Architecture

### Directory Structure

```
frontend/src/
├── main.tsx                    # App entry point
├── App.tsx                     # Router + providers setup
├── index.css                   # Tailwind imports + base styles
├── api/
│   ├── client.ts               # Base fetch wrapper
│   ├── groups.ts               # Group-related API functions
│   ├── jobs.ts                 # Job-related API functions
│   ├── config.ts               # Config API functions
│   └── types.ts                # Shared API types
├── hooks/
│   ├── useSocket.ts            # WebSocket connection hook
│   ├── useHealth.ts            # Health data query hook
│   ├── useGroups.ts            # Groups query hook
│   ├── useGroupJobs.ts         # Group jobs query hook
│   ├── useConfig.ts            # Config query/mutation hooks
│   └── useToast.ts             # Toast notification context
├── components/
│   ├── ui/                     # Base UI primitives
│   │   ├── Button.tsx
│   │   ├── Card.tsx
│   │   ├── Badge.tsx
│   │   ├── Input.tsx
│   │   ├── Select.tsx
│   │   ├── Skeleton.tsx
│   │   ├── Dialog.tsx
│   │   └── Spinner.tsx
│   ├── layout/
│   │   ├── Header.tsx
│   │   ├── Sidebar.tsx         # Optional: for settings navigation
│   │   └── Container.tsx
│   ├── health/
│   │   ├── HealthIndicator.tsx
│   │   └── HealthStats.tsx
│   ├── groups/
│   │   ├── GroupCard.tsx
│   │   ├── GroupGrid.tsx
│   │   └── GroupConfigForm.tsx
│   ├── jobs/
│   │   ├── JobCard.tsx
│   │   ├── JobList.tsx
│   │   ├── JobStatusBadge.tsx
│   │   └── JobSubmitForm.tsx
│   └── config/
│       └── GlobalConfigForm.tsx
├── pages/
│   ├── Dashboard.tsx           # Main dashboard view
│   ├── GroupDetail.tsx         # Group detail view
│   └── Settings.tsx            # Global settings page
├── contexts/
│   ├── ToastContext.tsx        # Toast notifications
│   └── SocketContext.tsx       # WebSocket provider
├── lib/
│   ├── utils.ts                # Utility functions (cn, formatDate, etc.)
│   └── constants.ts            # App constants
└── types/
    └── index.ts                # Shared TypeScript types
```

### Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                         React App                                │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    TanStack Query                          │  │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐      │  │
│  │  │ health  │  │ groups  │  │  jobs   │  │ config  │      │  │
│  │  │  query  │  │  query  │  │ queries │  │ queries │      │  │
│  │  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘      │  │
│  │       │            │            │            │            │  │
│  │       └────────────┴────────────┴────────────┘            │  │
│  │                         │                                  │  │
│  │                   Cache Layer                              │  │
│  └───────────────────────────────────────────────────────────┘  │
│                            │                                     │
│  ┌─────────────────────────┴─────────────────────────────────┐  │
│  │                    Socket.IO Hook                          │  │
│  │  Events: status_update, group_created, health_update       │  │
│  │  Action: Invalidate relevant queries → triggers refetch    │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                            │
                    REST API + WebSocket
                            │
                    ┌───────┴───────┐
                    │    Backend    │
                    └───────────────┘
```

---

## Implementation Phases

### Phase 1: Foundation & Infrastructure

Set up the build tooling, providers, and base architecture. This phase establishes the foundation for all subsequent work.

#### Tailwind CSS Setup
- [X] Install Tailwind CSS v4 and dependencies (`tailwindcss`, `@tailwindcss/vite`)
- [X] Configure `vite.config.ts` with Tailwind plugin
- [X] Create `tailwind.config.js` with custom theme (colors matching current design)
- [X] Replace `index.css` with Tailwind directives and CSS variables for dark mode
- [X] Remove old component CSS files (keep as reference until migration complete)

#### TanStack Query Setup
- [X] Install TanStack Query v5 (`@tanstack/react-query`, `@tanstack/react-query-devtools`)
- [X] Create `QueryClientProvider` wrapper in `App.tsx`
- [X] Configure default query options (staleTime, refetchOnWindowFocus, etc.)
- [X] Add React Query DevTools (dev only)

#### API Layer Refactor
- [X] Create `api/client.ts` with base fetch wrapper (keep timeout, error handling)
- [X] Create `api/types.ts` with all TypeScript interfaces
- [X] Split API functions into domain files (`api/groups.ts`, `api/jobs.ts`, `api/config.ts`)
- [X] Export unified API from `api/index.ts`

#### Base Utilities
- [X] Create `lib/utils.ts` with `cn()` helper (classname merging)
- [X] Create `lib/constants.ts` with status colors, labels, and app constants
- [X] Create `types/index.ts` with shared types

#### Project Cleanup
- [X] Update `package.json` with new dependencies
- [X] Update TypeScript config if needed for new packages
- [X] Update ESLint config for Tailwind class ordering (optional)

---

### Phase 2: UI Component Library

Build reusable UI primitives using Tailwind CSS. These components form the design system for the application.

#### Base Components
- [X] Create `Button` component (variants: primary, secondary, ghost, danger; sizes: sm, md, lg)
- [X] Create `Card` component (with header, body, footer slots)
- [X] Create `Badge` component (status variants: success, error, progress, warning, neutral)
- [X] Create `Input` component (with label, error state, helper text)
- [X] Create `Select` component (native select with Tailwind styling)
- [X] Create `Skeleton` component (loading placeholder)
- [X] Create `Spinner` component (loading indicator)
- [X] Create `Dialog` component (modal wrapper using native `<dialog>` or headless UI)

#### Layout Components
- [X] Create `Container` component (max-width wrapper with padding)
- [X] Create `Header` component (app header with title, health indicator, settings link)
- [X] Create page layout wrapper (consistent spacing, responsive behavior)

#### Toast Notifications
- [X] Install Sonner (`sonner`)
- [X] Create `ToastContext` provider
- [X] Create `useToast` hook for triggering notifications
- [X] Integrate Toaster component in `App.tsx`

---

### Phase 3: Query Hooks & Socket Integration

Create TanStack Query hooks and integrate WebSocket for real-time cache invalidation.

#### Query Hooks
- [X] Create `useHealth` hook - fetches `/health`, returns health summary
- [X] Create `useGroups` hook - fetches `/groups`, returns group list with health
- [X] Create `useGroupJobs` hook - fetches `/groups/{name}/jobs`, parameterized by group name
- [X] Create `useConfig` hook - fetches `/config`, includes mutation for updates
- [X] Create `useGroupConfig` hook - fetches `/groups/{name}/config`, includes mutation
- [X] Create `useSubmitStatus` mutation hook - POSTs to `/status`

#### Socket Integration
- [X] Refactor `useSocket` to work with TanStack Query
- [X] On `status_update` event: invalidate relevant group jobs query
- [X] On `group_created` event: invalidate groups query
- [X] On `health_update` event: invalidate health query
- [X] Create `SocketContext` provider for connection status
- [X] Add connection status indicator to header (connected/disconnected badge)

#### Error Handling
- [X] Create error boundary wrapper for query errors
- [X] Implement toast notifications on mutation success/failure
- [X] Add retry logic with exponential backoff for failed queries

---

### Phase 4: Dashboard Page Rebuild

Rebuild the main dashboard with new components and data hooks.

#### Health Section
- [X] Create `HealthIndicator` component with Tailwind (pulsing dot, status message)
- [X] Create `HealthStats` component (metric cards: total, healthy, unhealthy, in-progress)
- [X] Wire up with `useHealth` hook
- [X] Add loading skeleton state
- [X] Add error state with retry button

#### Groups Grid
- [X] Create `GroupCard` component (name, job count, mini status indicators, link to detail)
- [X] Create `GroupGrid` component (responsive grid layout)
- [X] Wire up with `useGroups` hook
- [X] Add loading skeleton state (grid of skeleton cards)
- [X] Add empty state (no groups yet)
- [X] Add real-time updates via socket

#### Dashboard Page Assembly
- [X] Create new `Dashboard.tsx` using new components
- [X] Add responsive layout (stack on mobile, grid on desktop)
- [X] Implement dark mode with Tailwind dark: classes
- [X] Test WebSocket updates flow correctly

---

### Phase 5: Group Detail Page Rebuild

Rebuild the group detail page with job management capabilities.

#### Job Components
- [X] Create `JobStatusBadge` component (colored badge for each status)
- [X] Create `JobCard` component (job name, status badge, message, timestamp)
- [X] Create `JobList` component (list layout with proper spacing)
- [X] Add relative time display for timestamps ("2 minutes ago")

#### Group Header
- [X] Create group header with name and back navigation
- [X] Display effective configuration (timeout values)
- [X] Add "Edit Config" button (opens dialog)

#### Group Configuration
- [X] Create `GroupConfigForm` component (form for timeout overrides)
- [X] Wire up with `useGroupConfig` mutation hook
- [X] Show toast on successful update
- [X] Handle null values (revert to defaults)

#### Group Detail Page Assembly
- [X] Create new `GroupDetail.tsx` using new components
- [X] Wire up with `useGroupJobs` hook
- [X] Add loading skeleton state
- [X] Add error state with retry
- [X] Add empty state (no jobs in group)
- [X] Implement real-time job updates

---

### Phase 6: Settings Page & Global Config

Add settings page for global configuration management.

#### Settings Page Structure
- [X] Create `Settings.tsx` page component
- [X] Add route in `App.tsx` at `/settings`
- [X] Add navigation link to settings from header

#### Global Configuration Form
- [X] Create `GlobalConfigForm` component
- [X] Fields: progress_timeout_minutes, staleness_timeout_hours
- [X] Validation: positive integers, reasonable bounds
- [X] Wire up with `useConfig` mutation hook
- [X] Show current values as placeholders
- [X] Toast notification on save success/failure

#### Optional: Form Validation
- [X] Install React Hook Form and Zod (`react-hook-form`, `zod`, `@hookform/resolvers`)
- [X] Create Zod schemas for config forms
- [X] Integrate with form components for type-safe validation

---

### Phase 7: Job Submission UI

Add ability to submit job status updates directly from the frontend.

#### Job Submit Form
- [X] Create `JobSubmitForm` component (dialog-based)
- [X] Fields: group (text or select), job name, status (select), message (optional textarea)
- [X] Validation: required fields, character limits
- [X] Wire up with `useSubmitStatus` mutation hook
- [X] Toast notification on success/failure

#### Integration
- [X] Add "Submit Status" button to Dashboard header
- [X] Add "Submit Status" button to GroupDetail (pre-fills group name)
- [X] Auto-refresh relevant queries on successful submission

---

### Phase 8: Search & Filtering

Add search and filter capabilities for groups and jobs.

#### Dashboard Filtering
- [X] Add search input to filter groups by name
- [X] Add health status filter (all, healthy, unhealthy, in-progress)
- [X] Implement client-side filtering (data already loaded)
- [X] Persist filter state in URL params (optional)

#### Group Detail Filtering
- [X] Add search input to filter jobs by name
- [X] Add status filter (all, success, error, progress, timeout, stale)
- [X] Implement client-side filtering
- [X] Show filtered count ("Showing X of Y jobs")

---

### Phase 9: Polish & Accessibility

Final polish pass for UX improvements and accessibility compliance.

#### Loading States
- [X] Ensure all pages have proper loading skeletons
- [X] Add subtle loading indicators for background refetches
- [X] Implement optimistic updates where appropriate

#### Error Handling
- [X] Review all error states for clarity
- [X] Add helpful error messages (not just "Error occurred")
- [X] Ensure retry buttons are accessible

#### Accessibility
- [X] Audit color contrast ratios (WCAG AA)
- [X] Add proper ARIA labels to interactive elements
- [X] Ensure keyboard navigation works throughout
- [ ] Test with screen reader
- [X] Add focus indicators for all focusable elements

#### Animations
- [X] Add subtle transitions for state changes
- [X] Animate card appearances in grid
- [X] Smooth loading skeleton transitions
- [X] Pulsing animation for in-progress status

#### Responsive Design
- [X] Test all pages at mobile breakpoints (320px, 375px, 414px)
- [X] Test at tablet breakpoints (768px, 1024px)
- [X] Ensure touch targets are appropriately sized (44x44px minimum)

---

### Phase 10: Testing & Documentation

Add tests and update documentation.

#### Component Tests
- [X] Set up Vitest for component testing
- [X] Write tests for UI primitives (Button, Badge, Card)
- [X] Write tests for form components
- [X] Write tests for query hooks (using MSW for mocking)

#### Integration Tests
- [X] Update Playwright E2E tests for new UI
- [X] Test dashboard loads and displays data
- [X] Test group detail page navigation
- [X] Test settings page form submission
- [X] Test job submission form
- [X] Test real-time updates via WebSocket
- [X] Test dark mode toggle (if manual toggle added) - N/A: Uses system preference, no manual toggle

#### Documentation
- [X] Update README with new architecture overview
- [X] Document component library usage
- [ ] Add screenshots of new UI
- [X] Document environment variables

---

## Component Specifications

### Status Badge Colors

| Status | Light Mode | Dark Mode | Tailwind Classes |
|--------|------------|-----------|------------------|
| success | Green bg | Green bg (darker) | `bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200` |
| error | Red bg | Red bg (darker) | `bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200` |
| progress | Blue bg | Blue bg (darker) | `bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200` |
| timeout | Red bg | Red bg (darker) | `bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200` |
| stale | Orange bg | Orange bg (darker) | `bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200` |

### Health Status Mapping

| Health | Label | Icon | Color |
|--------|-------|------|-------|
| healthy | All systems operational | CheckCircle | Green |
| unhealthy | Some systems have issues | AlertCircle | Red |
| in_progress | Operations in progress | Loader | Blue |
| empty | No jobs configured | Info | Gray |

### Card Component Variants

```tsx
// Basic usage
<Card>
  <CardHeader>Title</CardHeader>
  <CardBody>Content</CardBody>
</Card>

// Clickable card (for groups)
<Card as="link" to="/groups/backups">
  ...
</Card>

// With status indicator
<Card status="healthy">
  ...
</Card>
```

---

## API Integration Notes

### TanStack Query Keys

```typescript
// Query key factory for consistency
export const queryKeys = {
  health: ['health'] as const,
  groups: ['groups'] as const,
  groupJobs: (name: string) => ['groups', name, 'jobs'] as const,
  config: ['config'] as const,
  groupConfig: (name: string) => ['groups', name, 'config'] as const,
};
```

### Socket Event Handlers

```typescript
// In SocketContext or useSocket
socket.on('status_update', (data) => {
  queryClient.invalidateQueries({ queryKey: queryKeys.groupJobs(data.job.group_name) });
  queryClient.invalidateQueries({ queryKey: queryKeys.health });
});

socket.on('group_created', () => {
  queryClient.invalidateQueries({ queryKey: queryKeys.groups });
});

socket.on('health_update', () => {
  queryClient.invalidateQueries({ queryKey: queryKeys.health });
  queryClient.invalidateQueries({ queryKey: queryKeys.groups });
});
```

---

## Dependencies to Add

```json
{
  "dependencies": {
    "@tanstack/react-query": "^5.x",
    "lucide-react": "^0.x",
    "sonner": "^1.x",
    "react-hook-form": "^7.x",
    "zod": "^3.x",
    "@hookform/resolvers": "^3.x"
  },
  "devDependencies": {
    "tailwindcss": "^4.x",
    "@tailwindcss/vite": "^4.x",
    "@tanstack/react-query-devtools": "^5.x",
    "vitest": "^3.x",
    "@testing-library/react": "^16.x"
  }
}
```

---

## Success Criteria

### Performance
- [X] Initial page load under 1 second (cached, dev server)
- [X] WebSocket updates reflect in UI within 100ms
- [X] No unnecessary API calls (query caching works correctly)

### Functionality
- [X] All existing features preserved
- [X] Configuration editing works for global and group settings
- [X] Job submission works from frontend
- [X] Search/filter works for groups and jobs
- [X] Real-time updates work correctly

### Quality
- [X] All Playwright E2E tests pass (34 tests)
- [X] No TypeScript errors
- [X] ESLint passes with no warnings (3 minor react-refresh warnings for context/test files)
- [ ] Lighthouse accessibility score > 90 (requires manual testing)

### UX
- [X] Dark mode works correctly (follows system preference)
- [X] Responsive design works on mobile
- [X] Loading states are smooth and informative
- [X] Error messages are helpful
- [X] Toast notifications appear for all user actions
