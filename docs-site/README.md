# Gait Docs Site

Static Next.js site for GitHub Pages deployment.

## Local Development

```bash
cd docs-site
npm ci
npm run dev
```

## Build

```bash
cd docs-site
npm run build
```

Output is written to `docs-site/out/`.

## Content Sources

- `docs/**`
- `README.md`
- `SECURITY.md`
- `CONTRIBUTING.md`

The site ingests markdown from the repository and renders static docs routes.
