# StatShed Frontend

A modern, real-time status dashboard for monitoring cluster job operations. Built with React 19, TypeScript, and Tailwind CSS v4.

> This is one half of the [`statshed-server`](../README.md) monorepo. To run the
> **full stack** (web UI + backend) with one command, see the [root README](../README.md).
> The instructions below cover the frontend on its own.

## Technology Stack

| Component | Technology | Version |
|-----------|------------|---------|
| Framework | React | 19 |
| Build Tool | Vite | 6 |
| Language | TypeScript | 5.7 |
| Routing | React Router | 7 |
| Server State | TanStack Query | 5 |
| Styling | Tailwind CSS | 4 |
| Icons | Lucide React | - |
| Real-time | Socket.IO Client | 4 |
| Forms | React Hook Form + Zod | 7 / 3 |
| Notifications | Sonner | 1 |

## Architecture Overview

```
src/
├── main.tsx                    # App entry point
├── App.tsx                     # Router + providers setup
├── index.css                   # Tailwind imports + base styles
├── api/                        # API layer
│   ├── client.ts               # Base fetch wrapper with timeout
│   ├── types.ts                # API response types
│   ├── groups.ts               # Group endpoints
│   ├── jobs.ts                 # Job/status endpoints
│   ├── config.ts               # Configuration endpoints
│   ├── health.ts               # Health endpoint
│   └── index.ts                # Unified exports
├── hooks/                      # React hooks
│   ├── useSocket.ts            # WebSocket connection
│   ├── useHealth.ts            # Health data query
│   ├── useGroups.ts            # Groups query
│   ├── useGroupJobs.ts         # Group jobs query
│   ├── useConfig.ts            # Global config query/mutation
│   ├── useGroupConfig.ts       # Group config query/mutation
│   └── useSubmitStatus.ts      # Job status submission mutation
├── components/
│   ├── ui/                     # Base UI primitives
│   ├── layout/                 # Layout components
│   ├── health/                 # Health indicators
│   ├── groups/                 # Group components
│   ├── jobs/                   # Job components
│   └── config/                 # Configuration forms
├── pages/
│   ├── Dashboard.tsx           # Main dashboard view
│   ├── GroupDetail.tsx         # Group detail view
│   └── Settings.tsx            # Global settings page
├── contexts/
│   ├── ToastContext.tsx        # Toast notifications
│   └── SocketContext.tsx       # WebSocket provider
├── lib/
│   ├── utils.ts                # Utility functions (cn, formatDate)
│   └── constants.ts            # Status colors, query keys
├── types/
│   └── index.ts                # Shared TypeScript types
└── test/                       # Test setup and mocks
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

## Getting Started

### Prerequisites

- Node.js 18+
- npm, yarn, pnpm, or bun

### Installation

```bash
# Install dependencies
npm install

# Start development server
npm run dev
```

The development server runs on `http://localhost:7827` and proxies API requests to `http://localhost:7828`.

### Scripts

| Command | Description |
|---------|-------------|
| `npm run dev` | Start development server |
| `npm run build` | Type check and build for production |
| `npm run preview` | Preview production build |
| `npm run test` | Run unit tests with Vitest |
| `npm run test:e2e` | Run E2E tests with Playwright |
| `npm run lint` | Run ESLint |

### Docker

The frontend is normally built and run via the root compose file (which also wires
it to the backend) — from the repo root:

```bash
docker compose up --build -d        # frontend + backend
```

To build and run only this image by hand:

```bash
docker build -t statshed-frontend .
docker run -p 7827:80 -e BACKEND_URL=http://localhost:7828 statshed-frontend
```

The containerized frontend will be available at `http://localhost:7827`.

#### Configuring the Backend URL

Set the `BACKEND_URL` environment variable to point to your backend server. Edit `docker-compose.yml`:

```yaml
environment:
  - BACKEND_URL=http://app1.gen.realgo.com:7828
```

Or pass it directly when running the container:

```bash
docker run -p 7827:80 -e BACKEND_URL=http://app1.gen.realgo.com:7828 statshed-frontend
```

**Note:** The Docker build creates a production build served by nginx. Nginx proxies `/api` and `/socket.io` requests to the configured backend. For development with hot reload, use `npm run dev` instead.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_API_URL` | `http://localhost:7828` | Backend API URL (only needed in production) |

In development, the Vite dev server proxies `/api/*` requests to `http://localhost:7828` and `/socket.io/*` for WebSocket connections.

## Component Library

The UI components are located in `src/components/ui/` and follow a consistent API pattern.

### Button

Flexible button component with variants and sizes.

```tsx
import { Button } from '@/components/ui'

// Variants: primary (default), secondary, ghost, danger
<Button variant="primary">Save</Button>
<Button variant="secondary">Cancel</Button>
<Button variant="ghost">Details</Button>
<Button variant="danger">Delete</Button>

// Sizes: sm, md (default), lg
<Button size="sm">Small</Button>
<Button size="lg">Large</Button>

// Loading state
<Button isLoading>Saving...</Button>
```

**Props:**
- `variant`: `'primary' | 'secondary' | 'ghost' | 'danger'` (default: `'primary'`)
- `size`: `'sm' | 'md' | 'lg'` (default: `'md'`)
- `isLoading`: `boolean` - Shows spinner and disables button

### Card

Compound component for card layouts with optional link behavior.

```tsx
import { Card, CardHeader, CardBody, CardFooter } from '@/components/ui'

// Basic card
<Card>
  <CardHeader>Title</CardHeader>
  <CardBody>Content</CardBody>
  <CardFooter>Footer</CardFooter>
</Card>

// Clickable card (renders as Link)
<Card to="/groups/backups">
  <CardBody>Click to navigate</CardBody>
</Card>

// With health status indicator (colored top bar)
<Card status="healthy">
  <CardBody>Healthy group</CardBody>
</Card>
```

**Props:**
- `to`: `string` - If provided, renders as React Router Link
- `status`: `'healthy' | 'unhealthy' | 'in_progress' | 'empty'` - Shows colored indicator bar

### Badge

Status indicator badges for jobs and general use.

```tsx
import { Badge, JobStatusBadge } from '@/components/ui'

// General badge variants
<Badge variant="success">Completed</Badge>
<Badge variant="error">Failed</Badge>
<Badge variant="warning">Warning</Badge>
<Badge variant="progress">Running</Badge>
<Badge variant="neutral">Unknown</Badge>

// Job-specific badge (uses job status directly)
<JobStatusBadge status="success" />  // Shows "Success"
<JobStatusBadge status="error" />    // Shows "Error"
<JobStatusBadge status="progress" /> // Shows "In Progress"
<JobStatusBadge status="timeout" />  // Shows "Timeout"
<JobStatusBadge status="stale" />    // Shows "Stale"
```

### Input

Form input with label and error state support.

```tsx
import { Input } from '@/components/ui'

<Input
  label="Group Name"
  placeholder="Enter group name"
  error="This field is required"
  helperText="The name must be unique"
/>

// With React Hook Form
<Input
  label="Timeout"
  type="number"
  {...register('timeout')}
  error={errors.timeout?.message}
/>
```

**Props:**
- `label`: `string` - Label text above input
- `error`: `string` - Error message (shows red border and text)
- `helperText`: `string` - Helper text below input

### Select

Styled native select element.

```tsx
import { Select } from '@/components/ui'

<Select
  label="Status"
  value={status}
  onChange={(e) => setStatus(e.target.value)}
>
  <option value="success">Success</option>
  <option value="error">Error</option>
  <option value="progress">In Progress</option>
</Select>
```

### Dialog

Modal dialog using native `<dialog>` element.

```tsx
import Dialog from '@/components/ui/Dialog'

const [open, setOpen] = useState(false)

<Dialog
  open={open}
  onClose={() => setOpen(false)}
  title="Edit Configuration"
>
  <p>Dialog content here</p>
  <Button onClick={() => setOpen(false)}>Close</Button>
</Dialog>
```

**Props:**
- `open`: `boolean` - Controls visibility
- `onClose`: `() => void` - Called when dialog should close
- `title`: `string` - Dialog header title

### Skeleton

Loading placeholder components.

```tsx
import {
  Skeleton,
  SkeletonText,
  SkeletonTitle,
  SkeletonCard,
  SkeletonBadge
} from '@/components/ui'

// Base skeleton (customize height/width via className)
<Skeleton className="h-8 w-full" />

// Pre-styled variants
<SkeletonTitle />      // Wide title placeholder
<SkeletonText />       // Text line placeholder
<SkeletonBadge />      // Badge placeholder
<SkeletonCard />       // Full card placeholder
```

### Spinner

Loading indicator.

```tsx
import { Spinner } from '@/components/ui'

<Spinner />                    // Default size
<Spinner className="h-8 w-8" /> // Custom size
```

## Hooks

### useHealth

Fetches overall health status.

```tsx
import { useHealth } from '@/hooks'

function HealthDisplay() {
  const { data, isLoading, error } = useHealth()

  if (isLoading) return <Skeleton />
  if (error) return <ErrorMessage error={error} />

  return <span>Status: {data.status}</span>
}
```

### useGroups

Fetches all groups with health summary.

```tsx
import { useGroups } from '@/hooks'

function GroupList() {
  const { data, isLoading } = useGroups()

  return data?.groups.map(group => (
    <GroupCard key={group.id} group={group} />
  ))
}
```

### useGroupJobs

Fetches jobs for a specific group.

```tsx
import { useGroupJobs } from '@/hooks'

function JobList({ groupName }: { groupName: string }) {
  const { data, isLoading } = useGroupJobs(groupName)

  return data?.jobs.map(job => (
    <JobCard key={job.id} job={job} />
  ))
}
```

### useConfig / useGroupConfig

Configuration queries and mutations.

```tsx
import { useConfig, useGroupConfig } from '@/hooks'

// Global config
const { data: config, updateConfig } = useConfig()
await updateConfig({ progress_timeout_minutes: 10 })

// Group-specific config
const { data: groupConfig, updateConfig: updateGroupConfig } = useGroupConfig('backups')
await updateGroupConfig({ staleness_timeout_hours: 48 })
```

### useSubmitStatus

Submit job status updates.

```tsx
import { useSubmitStatus } from '@/hooks'

function SubmitButton() {
  const { mutate, isPending } = useSubmitStatus()

  const handleSubmit = () => {
    mutate({
      group: 'backups',
      job: 'db-backup',
      status: 'success',
      message: 'Backup completed'
    })
  }

  return <Button onClick={handleSubmit} isLoading={isPending}>Submit</Button>
}
```

## Dark Mode

The application supports dark mode automatically based on system preferences. Tailwind CSS v4 dark mode classes are used throughout:

- Light mode: Default styles
- Dark mode: `dark:` prefixed classes (e.g., `dark:bg-gray-800`)

The color scheme is controlled by the `prefers-color-scheme` media query in CSS.

## Testing

### Unit Tests

Unit tests use Vitest and React Testing Library. Mock Service Worker (MSW) is used for API mocking.

```bash
# Run tests
npm run test

# Run with coverage
npm run test -- --coverage

# Watch mode
npm run test -- --watch
```

### E2E Tests

End-to-end tests use Playwright.

```bash
# Run E2E tests
npm run test:e2e

# Run in UI mode
npx playwright test --ui

# Run specific test file
npx playwright test dashboard.spec.ts
```

## API Documentation

See [../docs/frontend-api.md](../docs/frontend-api.md) for complete REST API and WebSocket
documentation, and [../docs/restapi.md](../docs/restapi.md) for the full REST reference.

## License

Released under [CC0 1.0 Universal](../LICENSE) (public domain dedication).
