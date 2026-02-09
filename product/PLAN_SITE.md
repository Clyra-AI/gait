# PLAN SITE: GitHub Pages Docs + Marketing Site

Date: 2026-02-09  
Source of truth: `product/PRD.md`, `product/PLAN_v1.md`, `product/PLAN_v1.6.md`, `product/PLAN_v1.7.md`, current docs and codebase  
Scope: OSS-facing website only (docs + marketing). No hosted control-plane features.

This plan is execution-ready and designed to be implemented top-to-bottom with minimal interpretation.

---

## Objective

Ship a world-class, high-conversion, high-discoverability public site for Gait using GitHub Pages, while preserving technical accuracy from the current OSS codebase.

The site must do two jobs at once:

- explain and convert (marketing)
- document and unblock implementation (technical docs)

---

## Locked Decisions

- Deployment target: **GitHub Pages**
- Site location in repo: `docs-site/`
- Content source: existing markdown/docs in `docs/` plus selected root docs (`README.md`, `SECURITY.md`, `CONTRIBUTING.md`)
- Rendering model: static export build
- Search: **out of scope** for this version
- OSS/ENT boundary: mention packaging line, avoid enterprise implementation detail
- AEO baseline: provide `llms.txt`, machine-readable docs pointers, crawlable sitemap assets

---

## Current Baseline (Observed)

Already available:
- strong root positioning doc (`README.md`)
- broad docs map (`docs/README.md`)
- integration examples and contracts already implemented
- install, homebrew, UAT, hardening, and policy rollout docs in place

Current gaps:
- no dedicated user-facing docs/marketing site scaffold
- no Pages deployment workflow
- no structured AEO/LLM-facing content bundle for the public site surface
- discoverability currently depends on repo markdown navigation only

---

## Information Architecture (Target)

Primary navigation:

1. Home
2. Docs
3. Integrations
4. Security
5. OSS vs Enterprise
6. LLM Context

Docs sections:

- Start Here (install, demo, first runpack, verify)
- Core Flows (runpack, regress, gate, doctor)
- Production Rollout (policy rollout, approvals, defaults, SLO)
- Contracts (primitive, endpoint taxonomy, skill provenance)
- Operations (CI regress kit, UAT, packaging, homebrew)

---

## Epic S0: Site Foundation

### Story S0.1: Create docs-site app scaffold

Tasks:
- Create `docs-site/` with a static-export web app.
- Add shared layout with sidebar + header + content frame.
- Add global styles aligned with reference UX pattern.

Acceptance criteria:
- Local build produces static output directory.
- Site renders Home + Docs routes without runtime server dependencies.

### Story S0.2: Add docs ingestion layer

Tasks:
- Add markdown loader utilities for `docs/` and selected root docs.
- Generate static params for docs routes.
- Ensure broken slugs fail safely with 404.

Acceptance criteria:
- `docs/` files are rendered as navigable pages.
- Build fails clearly on malformed loader assumptions.

---

## Epic S1: Marketing + Conversion Surface

### Story S1.1: Build high-conversion homepage

Tasks:
- Add hero with one-line value proposition and concrete CTA paths.
- Add quickstart command block with first-win flow.
- Add feature grid tied to real implemented capabilities only.
- Add proof-oriented comparison table and action-oriented CTA footer.

Acceptance criteria:
- A new user can understand value and run first command from homepage alone.
- Every claim maps to an existing command/flow in current codebase.

### Story S1.2: Add product narrative pages

Tasks:
- Add “How it works” narrative in docs route.
- Add OSS boundary and packaging explanation page.
- Add integration landing page linking framework examples.

Acceptance criteria:
- Messaging is consistent with `README.md` and `docs/packaging.md`.
- No enterprise control-plane internals are exposed.

---

## Epic S2: Technical Docs Ladder

### Story S2.1: Publish canonical docs navigation

Tasks:
- Add deterministic navigation map for high-value docs.
- Prioritize start-to-production reading order.
- Link to contracts and runbooks as normative references.

Acceptance criteria:
- Docs map is coherent and non-duplicative.
- Reader can move from install -> demo -> production rollout without dead ends.

### Story S2.2: Include root governance docs

Tasks:
- Surface `SECURITY.md` and `CONTRIBUTING.md` in docs experience.
- Preserve original markdown as source of truth.

Acceptance criteria:
- External users can find governance docs from site navigation.

---

## Epic S3: SEO + AEO Packaging

### Story S3.1: Add crawl and canonical assets

Tasks:
- Add `robots.txt`.
- Add static `sitemap.xml`.
- Set canonical metadata and OpenGraph metadata in layout.

Acceptance criteria:
- Pages include title/description/canonical metadata.
- Robots and sitemap are publicly served in static output.

### Story S3.2: Add AI-discovery assets

Tasks:
- Add `llms.txt`.
- Add concise LLM context pages under `public/llm/`.
- Add AI-oriented sitemap pointer file.

Acceptance criteria:
- LLM context files are reachable and align with current OSS feature set.
- Content avoids speculative/unshipped claims.

---

## Epic S4: CI/CD for Site

### Story S4.1: Add Pages deployment workflow

Tasks:
- Add `.github/workflows/docs.yml` to build and deploy `docs-site/` to Pages.
- Trigger on docs/site/workflow changes and manual dispatch.

Acceptance criteria:
- Push to `main` with docs-site changes triggers deployment workflow.
- Successful run publishes artifact and deploys to Pages environment.

### Story S4.2: Add local site validation commands

Tasks:
- Add `Makefile` targets for docs-site install/build/check.
- Document local preview/build in docs.

Acceptance criteria:
- Contributors can validate site before push with one command path.

---

## Epic S5: Validation and Readiness

### Story S5.1: Validate links and rendering

Tasks:
- Build site locally and verify major routes.
- Validate markdown rendering for key docs.
- Validate Mermaid snippets render without syntax errors where present.

Acceptance criteria:
- No broken internal links in key user journeys.
- No route build failures for docs pages.

### Story S5.2: Release readiness

Tasks:
- Update root docs references where needed.
- Ensure no conflict with release and CI workflows.

Acceptance criteria:
- Existing CI remains green after site changes.
- Docs workflow deploys successfully.

---

## Definition of Done (Site)

- `docs-site/` exists and builds to static output.
- GitHub Pages workflow deploys successfully from `main`.
- Home page serves as both technical entrypoint and marketing conversion surface.
- Documentation ladder is coherent, codebase-aligned, and non-duplicative.
- SEO + AEO baseline assets are live (`robots.txt`, `sitemap.xml`, `llms.txt`).
- No search implementation is introduced.

---

## Out of Scope (This Plan)

- Hosted analytics dashboard
- Full blog/CMS platform
- On-site search
- Enterprise control-plane documentation details
- Net-new product capabilities not already in OSS codebase
