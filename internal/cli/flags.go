// Package cli wires flags, config, and pre-flight validation.
package cli

import (
	"os"
	"slices"
	"strconv"

	"github.com/spf13/cobra"
)

// Flags is the post-parse, post-env, post-config effective configuration
// for one agon run. See specs/04-cli-flags.md for field semantics.
type Flags struct {
	Main            string
	Side            string
	SideCount       int
	MainModel       string
	SideModel       string
	MaxTurn         int
	SessionID       string
	Transcript      string
	DiffFrom        string
	DiffTo          string
	TaskContext     string
	Judge           string
	CostCap         int
	ChangedLinesMin int
	StateDir        string
	Format          string
	Config          string
	Verbose         int
	LogMode         string
	// AllowStyleAttacks keeps style-shaped critic attacks instead of
	// dropping them. TOML-only (allow_style_attacks); no CLI flag.
	AllowStyleAttacks bool
}

// LogMode constants for --log-mode. See Flags.LogMode for the
// exact contract behind each value.
const (
	// LogModeSilent suppresses all per-round progress and the heartbeat.
	// Useful for CI runs where stderr should stay clean.
	LogModeSilent = "silent"
	// LogModeConcise is the default: one progress line per round +
	// a 10s heartbeat while an agent call is in flight.
	LogModeConcise = "concise"
	// LogModeVerbose is concise plus a faster heartbeat and per-call
	// stderr surfacing when the underlying CLI complained. Stops
	// short of stream-json "live thinking" - that lives behind a
	// separate, future flag.
	LogModeVerbose = "verbose"
)

// ValidLogModes lists the values --log-mode accepts; preflight uses
// it both for validation and for the error message.
var ValidLogModes = []string{LogModeSilent, LogModeConcise, LogModeVerbose}

// IsValidLogMode reports whether s is one of the accepted modes.
func IsValidLogMode(s string) bool {
	return slices.Contains(ValidLogModes, s)
}

// ShouldShowHelp reports whether a bare `agon` invocation should
// short-circuit to the help text instead of running through preflight
// and failing on a "cannot determine task context" message. The rule:
// no command-line arguments AND no env-supplied task source. With
// either of those present we honour the user's intent to actually
// run; only the empty-shoot case gets redirected to --help.
//
// argc is the length of os.Args (program path + arg count). The
// helper takes it as a parameter so tests don't have to mutate
// global state.
func ShouldShowHelp(argc int, f *Flags) bool {
	if argc > 1 {
		return false
	}
	return f.SessionID == "" && f.Transcript == "" && f.TaskContext == ""
}

// MaxRoundsFor translates the user-facing --max-turn (number of
// critic↔proposer pairs) into the internal round-cap the engine
// uses. One turn is two rounds: a critic message followed by the
// proposer's reply. Lives here so the doubling rule is unit-testable
// without standing up the full Engine.
func MaxRoundsFor(maxTurn int) int { return 2 * maxTurn }

// DefaultFlags returns the built-in defaults.
func DefaultFlags() *Flags {
	return &Flags{
		Main:            "claude",
		Side:            "codex",
		SideCount:       4,
		MaxTurn:         3,
		DiffFrom:        "HEAD",
		DiffTo:          ".",
		Judge:           "none",
		CostCap:         50000,
		ChangedLinesMin: 10,
		StateDir:        ".agon",
		Format:          "markdown",
		LogMode:         LogModeConcise,
	}
}

// Bind registers all flags onto cmd and returns a *Flags whose fields
// are populated when cmd runs. Bind does not validate.
func Bind(cmd *cobra.Command) *Flags {
	f := DefaultFlags()

	cmd.Flags().StringVar(&f.Main, "main", f.Main, "proposer agent: claude or codex (codex is v1)")
	cmd.Flags().StringVar(&f.Side, "side", f.Side, "critic agent: claude or codex")
	cmd.Flags().IntVar(&f.SideCount, "side-count", f.SideCount, "number of critic forks; each declares its own topic in R1")
	cmd.Flags().StringVar(&f.MainModel, "main-model", f.MainModel, "proposer model; required (and must differ from --side-model) when same family")
	cmd.Flags().StringVar(&f.SideModel, "side-model", f.SideModel, "critic model; required (and must differ from --main-model) when same family")
	cmd.Flags().IntVar(&f.MaxTurn, "max-turn", f.MaxTurn, "per-fork cap on critic↔proposer exchanges (1 turn = 1 critic message + 1 proposer reply)")
	cmd.Flags().StringVar(&f.SessionID, "session-id", f.SessionID, "claude root session id (claude-as-proposer auto-trigger)")
	cmd.Flags().StringVar(&f.Transcript, "transcript", f.Transcript, "path to root claude JSONL transcript")
	cmd.Flags().StringVar(&f.DiffFrom, "diff-from", f.DiffFrom, "git ref for diff base")
	cmd.Flags().StringVar(&f.DiffTo, "diff-to", f.DiffTo, "git ref for diff head; '.' means working tree")
	cmd.Flags().StringVar(&f.TaskContext, "task-context", f.TaskContext, "explicit task description; required iff neither --session-id nor --transcript is set")
	cmd.Flags().StringVar(&f.Judge, "judge", f.Judge, "none | llm | human (only 'none' is supported in v0)")
	cmd.Flags().IntVar(&f.CostCap, "cost-cap", f.CostCap, "total token budget across all forks")
	cmd.Flags().IntVar(&f.ChangedLinesMin, "changed-lines-min", f.ChangedLinesMin, "trivial-diff gate threshold")
	cmd.Flags().StringVar(&f.StateDir, "state-dir", f.StateDir, "where session folders live")
	cmd.Flags().StringVar(&f.Format, "format", f.Format, "summary format: markdown or json")
	cmd.Flags().StringVar(&f.Config, "config", f.Config, "explicit .agon.toml path; empty = search")
	cmd.Flags().CountVarP(&f.Verbose, "verbose", "v", "verbose: -v, -vv")
	cmd.Flags().StringVar(&f.LogMode, "log-mode", f.LogMode, "progress detail: silent | concise | verbose")

	return f
}

// ApplyEnv overrides any flag not explicitly set on cmd from the
// matching AGON_* env var. Only invoked for flags whose Changed bit
// is false.
func ApplyEnv(cmd *cobra.Command, f *Flags) {
	for _, b := range envBindings(f) {
		if cmd.Flags().Changed(b.flag) {
			continue
		}
		v := os.Getenv(b.env)
		if v == "" {
			continue
		}
		b.set(v)
	}
}

type envBinding struct {
	flag string
	env  string
	set  func(string)
}

func envBindings(f *Flags) []envBinding {
	return []envBinding{
		{"main", "AGON_MAIN", func(v string) { f.Main = v }},
		{"side", "AGON_SIDE", func(v string) { f.Side = v }},
		{"side-count", "AGON_SIDE_COUNT", func(v string) {
			if n, err := strconv.Atoi(v); err == nil {
				f.SideCount = n
			}
		}},
		{"main-model", "AGON_MAIN_MODEL", func(v string) { f.MainModel = v }},
		{"side-model", "AGON_SIDE_MODEL", func(v string) { f.SideModel = v }},
		{"max-turn", "AGON_MAX_TURN", func(v string) {
			if n, err := strconv.Atoi(v); err == nil {
				f.MaxTurn = n
			}
		}},
		{"session-id", "AGON_SESSION_ID", func(v string) { f.SessionID = v }},
		{"transcript", "AGON_TRANSCRIPT", func(v string) { f.Transcript = v }},
		{"diff-from", "AGON_DIFF_FROM", func(v string) { f.DiffFrom = v }},
		{"diff-to", "AGON_DIFF_TO", func(v string) { f.DiffTo = v }},
		{"task-context", "AGON_TASK_CONTEXT", func(v string) { f.TaskContext = v }},
		{"judge", "AGON_JUDGE", func(v string) { f.Judge = v }},
		{"cost-cap", "AGON_COST_CAP", func(v string) {
			if n, err := strconv.Atoi(v); err == nil {
				f.CostCap = n
			}
		}},
		{"changed-lines-min", "AGON_CHANGED_LINES_MIN", func(v string) {
			if n, err := strconv.Atoi(v); err == nil {
				f.ChangedLinesMin = n
			}
		}},
		{"state-dir", "AGON_STATE_DIR", func(v string) { f.StateDir = v }},
		{"format", "AGON_FORMAT", func(v string) { f.Format = v }},
		{"config", "AGON_CONFIG", func(v string) { f.Config = v }},
		{"verbose", "AGON_VERBOSE", func(v string) {
			if n, err := strconv.Atoi(v); err == nil {
				f.Verbose = n
			}
		}},
	}
}
