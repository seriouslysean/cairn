---
id: 8p6n
title: JSON output escapes <, > and & as \u003c, \u003e and \u0026
status: backlog
tags: [bug, area:json]
by: rue
created: 2026-10-18T20:23:18.892Z
updated: 2026-10-18T20:24:52.438Z
---
Go's `encoding/json` escapes `<`, `>` and `&` by default so the output is safe inside HTML. Cairn's JSON goes to agents, not browsers. `cairn help --json` gives `cairn edit \u003cid\u003e --status done`, and an issue body with `<` or `&` in it comes out the same way. It decodes to the same text, but anyone reading the raw output sees the escapes.

A `json.Encoder` with `SetEscapeHTML(false)` writes them as they are.
