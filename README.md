# Video List

Video List is a file manager with media streaming and video editing task support.
This project is designed to be easily deployed on remote devices such as NAS servers. It allows you to conveniently edit and manage video files stored remotely.

## Dependencies

- Go
- Node.js and npm
- FFmpeg (required for video editing)

## Build

```bash
git clone https://github.com/silmace/video-list.git
cd video-list
npm install
npm run build
go build -o video-list .
```

## Run

```bash
./video-list
```

Optional flags:

- `-baseDir`: override base directory at startup
- `-config`: set custom config file path (JSON)

Example:

```bash
./video-list -baseDir /data/media -config ./config/config.json
```

## Config File

- Default config path:
  - Windows: `%APPDATA%/video-list/config.json`
  - Linux/macOS: `~/.video-list/config.json`
- If the config file is missing or empty, it is created automatically on startup.

## GitHub Actions (Auto Build)

This repository includes CI workflow:

- `.github/workflows/ci-build.yml`

Trigger rules:

- `push`
- `pull_request`

CI steps:

1. Install Node dependencies
2. Build frontend (`npm run build`)
3. Compile backend (`go build .`)

## License

MIT