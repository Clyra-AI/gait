# Skill Submission Exports

This folder contains provider-specific submission copies generated from the core skills in `.agents/skills/`.

Generate or refresh variants:

```bash
python3 scripts/export_skill_submissions.py
```

Outputs:

- `./.agents/submissions/openai/<skill>/SKILL.md`
- `./.agents/submissions/anthropic/<skill>/SKILL.md`

Provider wording tweaks are appended automatically while preserving the same core workflow content.
