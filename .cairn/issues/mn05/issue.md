---
id: mn05
title: Say when a newer Cairn is out
status: backlog
tags: [enhancement, area:release]
by: rue
created: 2026-10-18T20:23:18.915Z
updated: 2026-10-18T20:23:18.915Z
---
Sean asked for "an update method", "or at least some way for it to check releases to see if it’s out of date". Today, updating means rerunning `go install github.com/seriouslysean/cairn/cmd/cairn@latest`. `cairn version` prints the installed version, and nothing says whether a newer one is out.

To settle before building:
- Where to look. The Go module proxy answers `https://proxy.golang.org/github.com/seriouslysean/cairn/@latest` with the newest tag, without GitHub or `gh`. The GitHub releases API is the other source. Either one is a network call, and Cairn makes none today.
- When to look: only on `cairn version`, on every command (a network call for every agent run), or once a day, with the result cached.
- Offline: the check must never fail or slow a command.
- Whether Cairn updates itself (for example, `cairn update` running `go install`), or only says what to run.

Sean put this after dogfooding.
