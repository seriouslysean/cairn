---
id: keeh
title: The README's install doesn't say the Go bin folder must be on PATH
status: backlog
tags: [bug, area:docs]
by: rue
created: 2026-10-18T20:23:18.881Z
updated: 2026-10-18T20:23:18.881Z
---
`go install` puts `cairn` in `go env GOBIN`, or in `$(go env GOPATH)/bin` when GOBIN is empty. When that folder isn't on PATH, the README's next line, `cairn version`, fails with "command not found". The Install section could say this in one line.
