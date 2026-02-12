# Gait Local UI Source

This folder contains the source project used to build static localhost UI assets.

## Development

```bash
cd ui/local
npm ci
npm run dev
```

## Build and sync embedded assets

```bash
bash scripts/ui_sync_assets.sh
```

This copies `ui/local/out/` into `internal/uiassets/dist/` for `gait ui` runtime serving.
