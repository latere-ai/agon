// Command agon is the adversarial review CLI for Claude Code coding
// sessions. The full design lives in specs/01-overview.md.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"latere.ai/x/agon/internal/agent"
	"latere.ai/x/agon/internal/cli"
	"latere.ai/x/agon/internal/ledger"
	"latere.ai/x/agon/internal/round"
	"latere.ai/x/agon/internal/state"
	"latere.ai/x/agon/internal/summary"
	"latere.ai/x/agon/pkg/adversarial/input"
)

// Set via -ldflags by goreleaser / Makefile.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// init populates version/commit/date from runtime build info when
// -ldflags didn't set them. This is the `go install` path: the
// toolchain stamps module version + vcs.revision into the binary
// even though -ldflags is empty. Without this, `go install ...@v0.0.1`
// would print "agon dev (none, unknown)".
func init() {
	if version != "dev" {
		return // ldflags wins; goreleaser / Makefile path
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		version = v
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
			if len(commit) > 7 {
				commit = commit[:7]
			}
		case "vcs.time":
			date = s.Value
		}
	}
}

func main() {
	os.Exit(realMain(os.Args[1:], os.Stdout, os.Stderr))
}

// realMain is the testable entry point: returns the process exit code
// instead of calling os.Exit, and accepts argv + stdout/stderr as
// parameters so tests can drive it without process spawning.
func realMain(args []string, stdout, stderr io.Writer) int {
	root := &cobra.Command{
		Use:           "agon",
		Short:         "Adversarial review for Claude Code coding sessions.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (%s, %s)", version, commit, date),
	}
	root.SetVersionTemplate("agon {{.Version}}\n")
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)

	flags := cli.Bind(root)
	var exitCode int
	root.RunE = func(cmd *cobra.Command, _ []string) error {
		_, err := cli.Effective(root, flags)
		if err != nil {
			return err
		}
		// Bare `agon` with no args and no env-supplied task source:
		// show help instead of failing preflight with "cannot
		// determine task context". A user who types just the binary
		// name expects orientation, not a cryptic error.
		if cli.ShouldShowHelp(len(args)+1, flags) {
			return cmd.Help()
		}
		plan, err := cli.Preflight(root.Context(), flags)
		if err != nil {
			return err
		}
		code, runErr := Run(root.Context(), flags, plan)
		exitCode = code
		return runErr
	}

	// Install the signal-aware context so SIGINT / SIGTERM cancel the
	// root context and propagate down through agent.Exec's process-group
	// teardown. Required by spec 21.
	ctx, cancel := round.InstallHandler(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := root.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(stderr, "agon:", err)
		return exitCodeFor(err)
	}
	return exitCode
}

// Run is the end-to-end orchestrator entry point. Exposed so e2e tests
// in this package can drive it directly without re-parsing flags.
// Returns the intended process exit code alongside any error: callers
// must propagate both so the binary's exit status matches the
// summary's verdict (0 clean, 1 unresolved leaves, etc.).
func Run(ctx context.Context, flags *cli.Flags, plan *cli.Plan) (int, error) {
	// Compute initial diff and gate.
	diff, err := input.Compute(ctx, input.DiffSpec{
		From: flags.DiffFrom, To: flags.DiffTo, Cwd: plan.Cwd,
	})
	if err != nil {
		return 1, err
	}
	// Auto-fallback: when the user didn't override the diff range and
	// the working tree is clean (claude already committed its changes),
	// agon the last commit instead of an empty diff. Manual invocation
	// after `git commit` is the common case where this matters.
	if diff.ChangedLines == 0 && flags.DiffFrom == "HEAD" && flags.DiffTo == "." {
		if fb, fbErr := input.Compute(ctx, input.DiffSpec{
			From: "HEAD~1", To: "HEAD", Cwd: plan.Cwd,
		}); fbErr == nil && fb.ChangedLines > 0 {
			fmt.Fprintln(os.Stderr, "[agon] working tree clean; falling back to HEAD~1..HEAD")
			diff = fb
			flags.DiffFrom, flags.DiffTo = "HEAD~1", "HEAD"
		}
	}
	if input.Trivial(diff, flags.ChangedLinesMin) {
		_ = state.AppendLog(plan.StateDirAbs, &state.LogRecord{
			TS: time.Now().UTC(), Kind: "skipped", Reason: "trivial-diff",
			ChangedLines: diff.ChangedLines, Threshold: flags.ChangedLinesMin,
		})
		fmt.Fprintf(os.Stderr, "[agon] skipped: trivial diff (%d changed lines < %d threshold)\n",
			diff.ChangedLines, flags.ChangedLinesMin)
		return 0, nil
	}

	// Resolve task context.
	taskCtx := flags.TaskContext
	if taskCtx == "" && (plan.TranscriptPath != "" || flags.SessionID != "") {
		if home, herr := os.UserHomeDir(); herr == nil {
			if tp, lerr := input.LocateTranscript(home, plan.Cwd, flags.SessionID, plan.TranscriptPath); lerr == nil {
				if t, rerr := input.ReadTranscript(tp); rerr == nil {
					taskCtx = t.FirstUser
				}
			}
		}
	}
	if taskCtx == "" {
		taskCtx = "(task context unavailable)"
	}

	// Open the session.
	sess, err := state.NewSession(plan.StateDirAbs, len(plan.Forks), time.Now())
	if err != nil {
		return 1, err
	}
	if err := state.WriteStart(sess, &state.StartFile{
		SessionID:   sess.ID,
		StartedAt:   sess.StartedAt,
		Proposer:    state.AgentRef{Agent: flags.Main, Model: flags.MainModel},
		Critic:      state.AgentRef{Agent: flags.Side, Model: flags.SideModel},
		TaskContext: taskCtx,
		TaskSource:  taskSource(flags),
		Diff: state.DiffSnap{
			From: flags.DiffFrom, To: flags.DiffTo,
			ChangedLines: diff.ChangedLines, Files: diff.Files,
			PatchPath: "diff.patch",
		},
		Config: state.ConfigSnap{
			MaxTurn: flags.MaxTurn, SideCount: flags.SideCount,
			CostCap: flags.CostCap, ChangedLinesMin: flags.ChangedLinesMin,
			Format:    flags.Format,
			MainModel: flags.MainModel, SideModel: flags.SideModel,
		},
		RootSession: state.RootSession{
			ID: flags.SessionID, TranscriptPath: plan.TranscriptPath, Cwd: plan.Cwd,
		},
		AgonVersion: version,
		GoVersion:   runtime.Version(),
	}); err != nil {
		return 1, err
	}
	if err := state.WriteRunDiff(sess, diff.Patch); err != nil {
		return 1, err
	}

	// Wire and run the engine. In verbose mode each agent driver
	// emits live tool / thinking events to stderr while a call is in
	// flight, so the operator sees what the agent is doing instead of
	// only "still running".
	verbose := flags.LogMode == cli.LogModeVerbose
	var eventOut io.Writer
	if verbose {
		eventOut = os.Stderr
	}
	proposer := &agent.ClaudeProposer{
		Cwd:      plan.Cwd,
		RootID:   flags.SessionID,
		Model:    flags.MainModel,
		Deadline: 5 * time.Minute,
		Verbose:  verbose,
		EventOut: eventOut,
	}
	criticFactory := func(_ int) agent.Critic {
		switch flags.Side {
		case "codex":
			return &agent.CodexCritic{Verbose: verbose, EventOut: eventOut}
		case "claude":
			return &agent.ClaudeCritic{Verbose: verbose, EventOut: eventOut}
		default:
			return agent.NewCritic(flags.Side)
		}
	}
	// Progress lines: per-fork/per-round status goes to stderr unless
	// --log-mode silent was passed.
	var progress io.Writer
	if flags.LogMode != cli.LogModeSilent {
		progress = os.Stderr
	}
	heartbeat := round.DefaultHeartbeatInterval
	if flags.LogMode == cli.LogModeVerbose {
		heartbeat = 5 * time.Second
	}
	eng := &round.Engine{
		Sess: sess, Cwd: plan.Cwd, ForkCount: len(plan.Forks),
		Proposer:  proposer,
		NewCritic: criticFactory,
		MaxRounds: cli.MaxRoundsFor(flags.MaxTurn), CostCap: flags.CostCap,
		TaskContext: taskCtx, DiffPatch: diff.Patch,
		Progress:          progress,
		HeartbeatInterval: heartbeat,
		Styled:            progress != nil && summary.IsTerminal(os.Stderr),
		AllowStyleAttacks: flags.AllowStyleAttacks,
	}
	sumRes, err := eng.Run(ctx)
	if err != nil {
		return 1, err
	}

	// Render summary + persist end.json.
	agg, _ := ledger.Aggregate(sess)
	decide := summary.Decide(sumRes)
	if err := summary.Persist(sumRes, agg, decide.ExitCode); err != nil {
		return 1, err
	}
	_ = state.AppendLog(plan.StateDirAbs, &state.LogRecord{
		TS: time.Now().UTC(), Kind: "run", Session: sess.ID,
		Termination: string(sumRes.Termination), Unresolved: sumRes.Unresolved,
		Tokens: sumRes.TokensUsed, WallSeconds: sumRes.WallSeconds,
		Summary: sess.Path("summary.md"),
	})
	// Print the rendered summary to stdout (TTY-styled when
	// interactive, plain markdown when piped).
	if body, readErr := os.ReadFile(sess.Path("summary.md")); readErr == nil {
		_, _ = summary.PrintRendered(os.Stdout, body, summary.IsTerminal(os.Stdout))
		fmt.Println()
	}
	if decide.StdoutLine != "" {
		fmt.Println(decide.StdoutLine)
	}

	return decide.ExitCode, nil
}

func taskSource(f *cli.Flags) string {
	switch {
	case f.TaskContext != "":
		return "flag"
	case f.Transcript != "":
		return "transcript"
	case f.SessionID != "":
		return "session-id-resume"
	default:
		return "unknown"
	}
}

func exitCodeFor(err error) int {
	var pe *cli.PreflightError
	if errors.As(err, &pe) && pe != nil {
		return pe.Code
	}
	return 1
}
