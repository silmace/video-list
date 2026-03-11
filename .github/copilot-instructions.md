# Copilot Instructions

## Project Overview

**video-list** is a self-hosted video file manager and editor. It consists of a Go HTTP server that embeds a compiled Vue.js frontend and exposes a REST API for browsing files, streaming media, and editing videos using FFmpeg.

## Repository Structure

```
.
├── server.go          # Go HTTP server (main entry point, package main)
├── ser1.go            # Older draft of the server (package m, not compiled into the binary)
├── src/               # Vue.js frontend source
│   ├── App.vue        # Root component
│   ├── main.ts        # Frontend entry point
│   ├── router/        # Vue Router configuration
│   ├── components/    # Vue components (FileList, VideoEditor, Navbar, PathBreadcrumb)
│   ├── composables/   # Vue composables (shared logic)
│   ├── types/         # TypeScript type definitions
│   └── assets/        # Static assets and styles
├── public/            # Public static files
├── dist/              # Built frontend output (embedded into Go binary)
├── package.json       # Node.js dependencies and scripts
├── vite.config.ts     # Vite build configuration
└── tsconfig*.json     # TypeScript configuration files
```

## Technology Stack

- **Backend**: Go (stdlib only — no frameworks; uses `net/http`, `embed`, `os/exec`)
- **Frontend**: Vue 3 + TypeScript + Vite + Vuetify 3 + Vue Router 4
- **Video Processing**: FFmpeg (called via `os/exec` in the Go server)
- **HTTP Client**: Axios (frontend)

## Build & Run

### Prerequisites
- Go 1.21+
- Node.js 18+ and npm
- FFmpeg installed and available in `$PATH`

### Development

```bash
npm install
npm run dev        # starts Vite dev server + Node.js server concurrently
```

### Production Build

```bash
npm install
npm run build      # builds Vue frontend into dist/ (runs vue-tsc + vite build)
go mod tidy
go build -o video-list ./server.go
./video-list -baseDir /path/to/your/media
```

## API Endpoints

All API routes are served by the Go backend under `/api/`:

| Method   | Path             | Description                                 |
|----------|------------------|---------------------------------------------|
| `GET`    | `/api/files`     | List files in a directory (`?path=<relPath>`) |
| `DELETE` | `/api/files`     | Delete a file or directory (`?path=<relPath>`) |
| `GET`    | `/api/media`     | Stream a media file (`?path=<relPath>`)     |
| `POST`   | `/api/edit-video`| Edit/trim/merge video segments (FFmpeg)     |

All file paths sent to the API are **relative paths** within `BaseDir`. The server always validates that resolved absolute paths remain within `BaseDir` to prevent directory traversal.

## Key Types

Defined in `src/types/index.ts` and mirrored as Go structs in `server.go`:

- `FileItem` / `FileInfo` — represents a file or directory entry
- `VideoSegment` / `Segment` — `startTime` and `endTime` strings in `HH:MM:SS` format
- `VideoEditPayload` / `VideoEditRequest` — video path and array of segments

## Coding Conventions

### Go (backend)
- Use the standard library; avoid introducing new external Go dependencies.
- Always validate and sanitize file paths using `toAbsolutePath()` before any filesystem operation. Never access files outside `BaseDir`.
- Log errors with `log.Println` or `log.Printf`; the server writes logs to `server.log`.
- Return JSON responses with appropriate HTTP status codes and a `Content-Type: application/json` header.

### Vue / TypeScript (frontend)
- Use the **Composition API** with `<script setup>` syntax.
- Use **Vuetify 3** components for all UI elements; follow Vuetify's theming system (dark/light theme toggled via `localStorage`).
- Type all data structures using interfaces from `src/types/index.ts`.
- Use **Axios** for HTTP requests to the backend API.
- Use `@vueuse/core` composables where appropriate for reactive browser APIs.

## Testing

There is currently no automated test suite. When adding tests:
- For Go: use the standard `testing` package and place test files alongside source files (`_test.go`).
- For the frontend: a Vitest setup would be consistent with the Vite-based build toolchain.
