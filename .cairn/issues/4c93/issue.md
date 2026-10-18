---
id: 4c93
title: new and edit can't read the body from a file or stdin
status: backlog
tags: [enhancement, area:cli]
by: rue
created: 2026-10-18T20:23:18.937Z
updated: 2026-10-18T20:23:18.937Z
---
`new` and `edit` take the body only as `--body text`. Agents write bodies of several paragraphs, with backticks and quotes in them, so each one goes through a heredoc and `--body "$(cat file)"`. That works, but quoting is where shell mistakes come from. `gh` has `--body-file`, and `-` there reads from stdin.

The case against: `"$(cat file)"` works in every shell, and a flag is more surface. Filing this tracker's first issues used the heredoc every time.
