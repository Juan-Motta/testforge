// Package cli implements the `testforge` command: a terminal and CI client of
// a TestForge workspace that authenticates with a workspace API key, plus the
// `testforge mcp` server for AI assistants.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"testforge/internal/client"
)

// Version is set at build time with -ldflags "-X testforge/internal/cli.Version=...".
var Version = "dev"

// Exit codes.
const (
	ExitOK     = 0
	ExitError  = 1 // usage, API or network error, timeout
	ExitFailed = 2 // runs finished but did not all pass
)

// Env is the process environment a command runs in.
type Env struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Getenv func(string) string
	// ReadSecret reads the API key without echo when stdin is a terminal;
	// nil reads the first line of Stdin.
	ReadSecret func() (string, error)
	// PollEvery is how often `runs wait` polls (default 3s).
	PollEvery time.Duration
	// ServeMCP runs the MCP server over stdio; set by main so the cli package
	// does not depend on the MCP SDK in tests that do not need it.
	ServeMCP func(ctx context.Context, c *client.Client, workspace func(context.Context) (string, error)) error
}

type globals struct {
	url       string
	workspace string
	json      bool
}

type command struct {
	name, args, summary string
	run                 func(ctx context.Context, e *Env, args []string) int
}

func commands() []command {
	return []command{
		{"login", "--url <api-url>", "Store an API key for an API URL (key from TESTFORGE_API_KEY or stdin)", cmdLogin},
		{"logout", "", "Remove the stored API key", cmdLogout},
		{"whoami", "", "Show the key's user, workspace and API URL", cmdWhoami},
		{"agents list", "", "List agents", cmdAgentsList},
		{"agents get", "<agent-id>", "Show an agent", cmdAgentsGet},
		{"scenarios list", "[--agent id] [--tag t] [--group id|none] [--q text]", "List scenarios", cmdScenariosList},
		{"scenarios get", "<scenario-id>", "Show a scenario", cmdScenariosGet},
		{"runs list", "[--batch id] [--scenario id] [--agent id] [--status s] [--limit n]", "List runs, newest first", cmdRunsList},
		{"runs get", "<run-id>", "Show a run: metrics, evaluation and transcript", cmdRunsGet},
		{"runs launch", "(--scenario id ... | --agent id | --tag t | --group id) [--wait]", "Launch a batch of runs", cmdRunsLaunch},
		{"runs wait", "--batch <batch-id>", "Wait for a batch's runs and evaluations; exit 2 unless all pass", cmdRunsWait},
		{"runs cancel", "<run-id>", "Cancel a queued or running run", cmdRunsCancel},
		{"runs judgement", "<run-id>", "Show a run's evaluation", cmdRunsJudgement},
		{"runs rejudge", "<run-id>", "Evaluate a run again", cmdRunsRejudge},
		{"reports", "[--days 7|30|90] [--agent id] [--scenario id]", "Evaluation report", cmdReports},
		{"mcp", "", "Serve the Model Context Protocol over stdio for AI assistants", cmdMCP},
		{"version", "", "Print the version", cmdVersion},
	}
}

// Main runs the command line and returns the exit code.
func Main(ctx context.Context, args []string, e *Env) int {
	if e.PollEvery <= 0 {
		e.PollEvery = 3 * time.Second
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		usage(e.Stdout)
		return ExitOK
	}
	for _, c := range commands() {
		words := strings.Fields(c.name)
		if len(args) >= len(words) && strings.Join(args[:len(words)], " ") == c.name {
			return c.run(ctx, e, args[len(words):])
		}
	}
	// A known group without a valid subcommand.
	for _, group := range []string{"agents", "scenarios", "runs"} {
		if args[0] == group {
			fmt.Fprintf(e.Stderr, "testforge %s: unknown or missing subcommand\n\n", group)
			usage(e.Stderr)
			return ExitError
		}
	}
	fmt.Fprintf(e.Stderr, "testforge: unknown command %q\n\n", args[0])
	usage(e.Stderr)
	return ExitError
}

func usage(w io.Writer) {
	fmt.Fprint(w, `TestForge CLI: test voice and text agents from a terminal, CI or an AI assistant.

Usage: testforge <command> [flags]

Commands:
`)
	for _, c := range commands() {
		fmt.Fprintf(w, "  %-16s %s\n", c.name, c.summary)
	}
	fmt.Fprint(w, `
Every command takes --url, --workspace and --json; run "testforge <command> -h" for its flags.

Authentication: a workspace API key (Settings → API keys in the web app).
  testforge login --url https://testforge.example.com     # key from TESTFORGE_API_KEY or stdin
Or set TESTFORGE_URL and TESTFORGE_API_KEY (CI). Flags win over the environment,
which wins over the stored login; the stored key is only sent to the stored URL.

Exit codes: 0 success, 1 error, 2 runs finished but not all passed (runs launch --wait, runs wait).
`)
}

// newFlags returns a flag set with the shared flags.
func newFlags(e *Env, name, args, summary string) (*flag.FlagSet, *globals) {
	fs := flag.NewFlagSet("testforge "+name, flag.ContinueOnError)
	fs.SetOutput(e.Stderr)
	g := &globals{}
	fs.StringVar(&g.url, "url", "", "API URL (default: TESTFORGE_URL or the stored login)")
	fs.StringVar(&g.workspace, "workspace", "", "workspace slug (default: TESTFORGE_WORKSPACE or the key's workspace)")
	fs.BoolVar(&g.json, "json", false, "print the API's JSON")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: testforge %s %s\n\n%s.\n\nFlags:\n", name, args, summary)
		fs.PrintDefaults()
	}
	return fs, g
}

// parse parses flags placed before or after positional arguments.
func parse(fs *flag.FlagSet, args []string) ([]string, int, bool) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, ExitOK, false
			}
			return nil, ExitError, false
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, 0, true
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

// parseExact parses and requires exactly n positional arguments.
func parseExact(e *Env, fs *flag.FlagSet, args []string, n int) ([]string, int, bool) {
	pos, code, ok := parse(fs, args)
	if !ok {
		return nil, code, false
	}
	if len(pos) != n {
		fmt.Fprintf(e.Stderr, "%s: expected %d argument(s), got %d\n", fs.Name(), n, len(pos))
		fs.Usage()
		return nil, ExitError, false
	}
	return pos, 0, true
}

// fail prints an error and returns ExitError.
func fail(e *Env, err error) int {
	var apiErr *client.Error
	if errors.As(err, &apiErr) && apiErr.Status == 401 {
		fmt.Fprintf(e.Stderr, "error: %s (the API key is invalid, expired or revoked)\n", apiErr.Error())
		return ExitError
	}
	fmt.Fprintf(e.Stderr, "error: %s\n", err)
	return ExitError
}

// workspaceOf returns the target's workspace, asking the API for the key's
// workspace when none was configured.
func (t *target) workspaceOf(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.workspace != "" {
		return t.workspace, nil
	}
	raw, err := t.client.Me(ctx)
	if err != nil {
		return "", err
	}
	var me meView
	if err := json.Unmarshal(raw, &me); err != nil {
		return "", err
	}
	if len(me.Workspaces) == 0 {
		return "", errors.New("the API key reaches no workspace")
	}
	t.workspace = me.Workspaces[0].Slug
	return t.workspace, nil
}

// connect resolves the target and its workspace.
func connect(ctx context.Context, e *Env, g *globals) (*target, string, error) {
	t, err := e.resolve(g)
	if err != nil {
		return nil, "", err
	}
	ws, err := t.workspaceOf(ctx)
	if err != nil {
		return nil, "", err
	}
	return t, ws, nil
}

// printJSON writes raw API JSON indented.
func printJSON(w io.Writer, raw json.RawMessage) {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		fmt.Fprintln(w, string(raw))
		return
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Fprintln(w, string(b))
}

func readKey(e *Env) (string, error) {
	if k := strings.TrimSpace(e.Getenv("TESTFORGE_API_KEY")); k != "" {
		return k, nil
	}
	if e.ReadSecret != nil {
		fmt.Fprint(e.Stderr, "API key: ")
		k, err := e.ReadSecret()
		fmt.Fprintln(e.Stderr)
		return strings.TrimSpace(k), err
	}
	line, err := bufio.NewReader(e.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
