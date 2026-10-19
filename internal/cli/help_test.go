package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

var helpUsage = []string{
	`init`,
	`new "title" [--body text] [--tag t]... [--milestone m] [--due YYYY-MM-DD]`,
	`edit <id> [--title t] [--body text] [--status s] [--tag +t|-t]... [--milestone m] [--due YYYY-MM-DD]`,
	`rm <id>`,
	`show <id> [--retracted] [--branches]`,
	`ls [--status s] [--tag t]... [--milestone m] [--due-before YYYY-MM-DD] [--branches]`,
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

var helpRules = []string{
	"Run cairn search before cairn new: Cairn does not look for duplicates.",
	"Comment on an issue someone else owns rather than editing it: comments never conflict across branches.",
	"evidence needs --source, decision quotes the owner, result says what was done.",
	"To close an issue, add a result comment, then cairn edit <id> --status done.",
	"Retract a wrong comment instead of deleting it. purge is only for mistakes like a pasted secret.",
}

// help is the only documentation an agent with just the binary has, so it gives every command's usage and the rules.
func TestHelpText(t *testing.T) {
	empty := &tracker{t: t, dir: t.TempDir(), now: t0, rnd: &seq{}}
	want := "Usage: cairn <command> [args]\n" +
		"Every command takes --json. Writes take --as (or CAIRN_ACTOR), --persona and --model.\n\n" +
		"Commands:\n  " + strings.Join(helpUsage, "\n  ") + "\n\n" +
		"Rules:\n  " + strings.Join(helpRules, "\n  ") + "\n"
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}} {
		if code, out, errOut := empty.run(args...); code != 0 || out != want || errOut != "" {
			t.Fatalf("%q: exit %d, %q\ngot\n%s\nwant\n%s", args, code, errOut, out, want)
		}
	}
}

func TestHelpJSON(t *testing.T) {
	empty := &tracker{t: t, dir: t.TempDir(), now: t0, rnd: &seq{}}
	code, data, _ := empty.json("help")
	var got map[string][]string
	if err := json.Unmarshal(data, &got); code != 0 || err != nil {
		t.Fatalf("exit %d, %s, %v", code, data, err)
	}
	want := map[string][]string{"usage": helpUsage, "rules": helpRules}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}
