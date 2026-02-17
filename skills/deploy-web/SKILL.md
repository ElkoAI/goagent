---
name: deploy-web
version: 1.0.0
description: Deploy files to a remote web host via rsync over SSH
metadata: {"openclaw":{"emoji":"🚀","requires":{"bins":["ssh"],"anyBins":["rsync","scp"],"env":["DEPLOY_HOST","DEPLOY_USER"]}}}
---

# Deploy to Web Host

You can deploy files to the user's web host using the `deploy-web.sh` script.

## Usage

```bash
./scripts/deploy-web.sh [OPTIONS] <local-path> [remote-subpath]
```

## Options

- `--dry-run` — preview what would be transferred without actually doing it
- `--delete` — remove remote files that no longer exist locally (use with caution)

## Common Tasks

### Deploy today's market report
```bash
./scripts/deploy-web.sh ~/.goagent/home/john/reports/daily/ reports/daily/
```

### Deploy a single file to the web root
```bash
./scripts/deploy-web.sh ./public/index.html
```

### Deploy an entire directory
```bash
./scripts/deploy-web.sh ./public/ www/
```

### Preview before deploying
Always offer a `--dry-run` first when the user hasn't deployed before or the scope is large:
```bash
./scripts/deploy-web.sh --dry-run ./public/ www/
```

## Environment Variables

These must be set in `.env`:
- `DEPLOY_HOST` — remote hostname
- `DEPLOY_USER` — SSH username
- `DEPLOY_PATH` — remote base path (default: `/`)
- `DEPLOY_PORT` — SSH port (default: `22`)

## Guidelines

- Always confirm with the user before deploying if this is a first-time deploy or if `--delete` is involved.
- Prefer `--dry-run` first to show the user what will change.
- The script falls back to `scp` if `rsync` is not available on the system.
- Remote file permissions are set to 755 for directories and 644 for files.
