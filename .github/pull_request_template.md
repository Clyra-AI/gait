## Summary

## Testing

- [ ] `make fmt`
- [ ] `make lint`
- [ ] `make test`

## Hardening Review

- [ ] Failure classification impact reviewed (`error_category`, `error_code`, retryability)
- [ ] Exit code contract impact reviewed (no accidental contract break)
- [ ] Deterministic output impact reviewed (`--json`, artifacts, golden fixtures)
- [ ] Security/privacy impact reviewed (secrets redaction, key handling, fail-closed behavior)

## Operational Notes

- [ ] User-facing docs updated where behavior changed (`README.md`, `docs/hardening/*`, runbooks)
- [ ] Docs sync checked across onboarding surfaces (`README.md`, `docs/README.md`, `docs-site/public/llm/*`, `docs-site/public/llms.txt`)
- [ ] New docs pages added to discoverability surfaces (`docs-site/src/lib/navigation.ts`, `docs-site/src/app/docs/page.tsx`, `docs-site/public/sitemap.xml`)
- [ ] Terminology changes reconciled in canonical definition pages (tool boundary + capability taxonomy)
- [ ] If agent behavior touched production-like paths, attached runpack evidence (`If your agent touched prod, attach the runpack.`)
- [ ] Included ticket footer evidence line (`gait run receipt --from <run_id|path>`)
- [ ] For new/expanded official integration lane proposals, attached `gait-out/integration_lane_scorecard.json` evidence and decision outcome
