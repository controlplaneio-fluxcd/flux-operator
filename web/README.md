# Flux Status - Frontend

Single-page application built with [Preact](https://preactjs.com/) (lightweight React alternative),
[Preact Signals](https://preactjs.com/guide/v10/signals/) for reactive state management,
[Tailwind CSS](https://tailwindcss.com/) for styling,
and [Vite](https://vite.dev/) as the build tool and dev server.

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

## Go Backend

- **Server setup**: `internal/web/serve.go` (HTTP server with graceful shutdown)
- **API routes**: `internal/web/router.go` registers HTTP handlers for `/api/v1/`
- **Embedded frontend**: Embeds `web/dist` in `web/embed.go` and serves it in `internal/web/fs.go`

## Mock Data

Mock data for UI development is organized in `src/mock/`:

- `report.js` - FluxReport mock data (`mockReport`)
- `events.js` - Events API mock data (`mockEvents`)
- `resources.js` - Resources API mock data (`mockResources`)

When `VITE_USE_MOCK_DATA=true`, the app uses dynamic imports to load mock data only when needed,
preventing mock data from being bundled in production builds.

**Important**: Keep mock data in sync with the real API shape!

## Testing

The project uses [Vitest](https://vitest.dev/) as the test framework
with [jsdom](https://github.com/jsdom/jsdom) for DOM simulation
and [@testing-library/preact](https://testing-library.com/docs/preact-testing-library/intro/) for component testing.

Run tests with coverage:

```bash
cd web && npm test -- --coverage
```

Test files and `vitest.setup.js` automatically have access
to Vitest globals (`describe`, `it`, `expect`, `vi`, etc.) without needing to import them.
This is configured in `eslint.config.js`.
