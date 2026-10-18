---
id: 9p27
title: ls lists done issues, with no way to hide them
status: backlog
tags: [enhancement, area:cli]
by: rue
created: 2026-10-18T20:23:18.959Z
updated: 2026-10-18T20:23:18.959Z
---
`cairn ls` lists every issue, including done ones, in the order they were created. `--status` takes one status, so there's no way to ask for everything that isn't done. With a few issues that's fine. Once most issues are done, the default list is mostly old work, and an agent looking for open work has to filter the output itself.

Options: leave done issues out unless `--all` is given (the way `show` treats retracted comments), or let `--status` take more than one value.
