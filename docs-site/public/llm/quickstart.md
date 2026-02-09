# Gait Quickstart

```bash
curl -fsSL https://raw.githubusercontent.com/davidahmann/gait/main/scripts/install.sh | bash
gait demo
gait verify run_demo
gait regress init --from run_demo --json
gait regress run --json --junit ./gait-out/junit.xml
```

Use `gait policy test` and `gait gate eval` to enforce high-risk tool-call boundaries.
