---
id: p5t3
title: The tag group error doesn't say where groups are set
status: backlog
tags: [bug, area:cli]
by: rue
created: 2026-10-18T20:23:18.948Z
updated: 2026-10-18T20:23:18.948Z
---
`cairn edit <id> --tag kind:bug` fails with `tag group "kind" is not in the config`. There's no command to add a group, which is fine, but the message doesn't say where groups are set: `tag_groups` in `.cairn/config.yml`. Naming that file and key in the message would save a search.
