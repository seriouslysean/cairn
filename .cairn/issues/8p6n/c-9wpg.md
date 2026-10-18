---
id: c-9wpg
kind: evidence
by: rue
at: 2026-10-18T20:23:44.662Z
source: internal/cli/cli.go:1665
---
successJSON uses json.Marshal, which always escapes HTML characters.
