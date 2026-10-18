---
id: jjdw
title: Times vary in width, so they don't sort as text
status: backlog
tags: [bug, area:storage]
by: rue
created: 2026-10-18T20:24:23.543Z
updated: 2026-10-18T20:24:23.543Z
---
Times are cut to milliseconds and written with `time.RFC3339Nano`, which drops trailing zeros. On mn05, comment c-kx7v has `at: 2026-10-18T20:23:44.7Z` while c-fwhn has `at: 2026-10-18T20:23:44.693Z`, and a time on a whole second has no fraction at all. The spec allows this, and Cairn parses times before it sorts, so its own order is right.

Anything that compares the strings gets the order wrong, though. `20:23:44.7Z` sorts after `20:23:44.71Z`, but it's 10 ms earlier. `jq 'sort_by(.at)'` on `show --json` does exactly that. A fixed three-digit fraction, layout `2006-01-02T15:04:05.000Z07:00`, sorts the same as text and as time, and still parses as RFC 3339. Files written before the change would stay valid, but they'd still have mixed widths.
