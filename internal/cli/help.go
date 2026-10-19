package cli

import "strings"

func runVersion(c *call) (output, error) {
	return output{versionJSON{Version: c.env.Version}, "cairn " + c.env.Version + "\n"}, nil
}

func runHelp(*call) (output, error) {
	const indent = "\n  "
	text := "Usage: cairn <command> [args]\n" +
		"Every command takes --json. Writes take --as (or " + actorEnv + "), --persona and --model.\n\n" +
		"Commands:" + indent + strings.Join(commandUsage, indent) + "\n\n" +
		"Rules:" + indent + strings.Join(agentRules, indent) + "\n"
	return output{helpJSON{Usage: commandUsage, Rules: agentRules}, text}, nil
}

// commandUsage is one usage line per command, in the order help prints them.
var commandUsage = []string{
	`init`,
	`new "title" [--body text] [--tag t]... [--milestone m] [--due YYYY-MM-DD]`,
	`edit <id> [--title t] [--body text] [--status s] [--tag +t|-t]... [--milestone m] [--due YYYY-MM-DD]`,
	`rm <id>`,
	`show <id> [--all]`,
	`ls [--status s] [--tag t]... [--milestone m] [--due-before YYYY-MM-DD]`,
	`comment <id> --kind evidence|decision|result [--source s] "text"`,
	`retract <id> <comment> --reason "text" [--disproved-by <comment>]`,
	`purge <id> <comment>`,
	`tags`,
	`milestones`,
	`milestone new <id> "title" [--due YYYY-MM-DD]`,
	`milestone edit <id> [--title t] [--status s] [--due YYYY-MM-DD]`,
	`search <words>...`,
	`version`,
	`help`,
}

// agentRules are the rules for using Cairn that docs/spec.md gives, word for word.
var agentRules = []string{
	"Run cairn search before cairn new: Cairn does not look for duplicates.",
	"Comment on an issue someone else owns rather than editing it: comments never conflict across branches.",
	"evidence needs --source, decision quotes the owner, result says what was done.",
	"To close an issue, add a result comment, then cairn edit <id> --status done.",
	"Retract a wrong comment instead of deleting it. purge is only for mistakes like a pasted secret.",
}
