package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// JSON goes to agents, not to a browser, so <, > and & come out as they are, not as <, > and &.
func TestJSONWritesCharactersAsIs(t *testing.T) {
	tr := newTracker(t)
	id := tr.issue("new", "title", "--body", "a < b && c > d").ID

	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"help", "--json"}, "cairn edit <id> --status done"},
		{[]string{"show", id, "--json"}, `"a < b && c > d\n"`},
		{[]string{"nosuch<&>", "--json"}, `unknown command \"nosuch<&>\"`},
	} {
		_, out, _ := tr.run(c.args...)
		if !strings.Contains(out, c.want) || strings.Contains(out, `\u00`) {
			t.Errorf("%q: want %s as is, got\n%s", c.args, c.want, out)
		}
	}
}

// --json before the command is not a command: the usage error still comes as the JSON envelope.
func TestJSONBeforeTheCommand(t *testing.T) {
	tr := newTracker(t)
	code, out, errOut := tr.run("--json", "ls")
	var e struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &e); err != nil || code != 2 || e.OK || e.Error.Code != "usage" ||
		errOut == "" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}
