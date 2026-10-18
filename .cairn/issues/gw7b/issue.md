---
id: gw7b
title: Per-command help is a usage error
status: backlog
tags: [bug, area:cli]
by: rue
created: 2026-10-18T20:23:18.868Z
updated: 2026-10-18T20:23:18.868Z
---
`cairn new --help` prints `cairn: unknown flag --help` and `cairn help new` prints `cairn: help takes no arguments`. Both exit 2, and neither says to run `cairn help`, which is where the usage is. An agent's first two guesses at help both fail.

The smallest fix: the error for `--help` or `-h` after a command says to run `cairn help`. Bigger: `cairn help <command>` and `<command> --help` print that command's usage line.
