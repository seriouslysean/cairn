---
id: 8hts
title: The spec lists commands that aren't built, without saying so
status: backlog
tags: [bug, area:docs]
by: rue
created: 2026-10-18T20:23:18.904Z
updated: 2026-10-18T20:23:18.904Z
---
The Commands table in `docs/spec.md` lists `cairn board`, `cairn pull`, `cairn push`, `cairn search --meaning`, and `--branches` on `ls` and `show`, beside the commands that exist. They're part of the v0 spec and come in later steps of `docs/plan.md`, but an agent that reads the table and tries them gets exit 2. `cairn help` lists only what's built.

Options: mark each of those rows "not built yet", or list them in a separate table.
