package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"latere.ai/x/agon/pkg/adversarial/input"
)

// Plan summarizes what one agon run will do; produced by Preflight.
type Plan struct {
	Cwd            string
	Forks          []ForkPlan
	StateDirAbs    string
	SessionID      string
	TranscriptPath string
}

// ForkPlan binds a 1-based critic index to its slot. The topic each
// fork attacks on is declared by the critic itself in R1; preflight
// only allocates the fork count.
type ForkPlan struct {
	Index int
}

// PreflightError carries the exit code the CLI should propagate.
type PreflightError struct {
	Code int
	Msg  string
	Wrap error
}

func (e *PreflightError) Error() string {
	if e.Wrap != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Wrap)
	}
	return e.Msg
}

func (e *PreflightError) Unwrap() error { return e.Wrap }

// Preflight runs every pre-flight check against f. On success it
// returns *Plan; on failure a *PreflightError.
func Preflight(_ context.Context, f *Flags) (*Plan, error) {
	// cwd resolution.
	cwd, err := os.Getwd()
	if err != nil {
		return nil, &PreflightError{Code: 101, Msg: "cannot resolve cwd", Wrap: err}
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return nil, &PreflightError{Code: 101, Msg: "cannot make cwd absolute", Wrap: err}
	}
	if f.Transcript != "" {
		// Compare in encoded space, not decoded. Decoding is lossy
		// because claude maps both `/` and `.` to `-`, so a path
		// containing a dot (e.g. /Users/x/dev/foo.bar/repo) decodes
		// ambiguously and would falsely flag a real match. The
		// --session-id branch below has always done this right;
		// this branch used to use the lossy decoder and rejected
		// any cwd with a `.` in it.
		if expected := encodedSegmentFromTranscript(f.Transcript); expected != "" {
			ours := input.EncodeCwd(cwd)
			if expected != ours {
				return nil, &PreflightError{
					Code: 101,
					Msg: fmt.Sprintf(
						"--transcript points at a session under projects/%s but the current cwd encodes to %s; "+
							"the original directory is approximately %q (claude's encoding maps both `/` and `.` to `-` so this hint may need a `.` or a `-` swapped); "+
							"cd there and rerun (claude --resume is cwd-scoped)",
						expected, ours, input.DecodeCwd(expected)),
				}
			}
		}
	} else if f.SessionID != "" {
		// claude's --resume is cwd-scoped. We scan ~/.claude/projects/*
		// only as a best-effort fast-fail: when the session file is at
		// the documented layout AND its encoded directory disagrees
		// with our cwd's encoded form, exit 101. Comparing in encoded
		// space is reliable; decoding back to a path is lossy because
		// claude maps both `/` and `.` to `-`. If the layout changes,
		// the home dir is unreadable, or the session genuinely is not
		// there, we fall through and let the actual claude --resume
		// call surface the real error 30-90s later. The check never
		// introduces a new failure mode; it only converts a slow-fail
		// into a fast-fail on the documented layout.
		if home, herr := os.UserHomeDir(); herr == nil {
			if _, foundSeg, ferr := input.FindSession(home, f.SessionID); ferr == nil {
				ourSeg := input.EncodeCwd(cwd)
				if foundSeg != ourSeg {
					return nil, &PreflightError{
						Code: 101,
						Msg: fmt.Sprintf(
							"--session-id %s lives under projects/%s but the current cwd encodes to %s; "+
								"the original directory is approximately %q (claude's encoding maps both `/` and `.` to `-` so this hint may need a `.` or a `-` swapped); "+
								"cd there and rerun (claude --resume is cwd-scoped)",
							f.SessionID, foundSeg, ourSeg, input.DecodeCwd(foundSeg)),
					}
				}
			}
		}
	}

	// 3. Env hygiene - never an error; informational only.
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		_ = os.Unsetenv("ANTHROPIC_API_KEY")
		if f.Verbose >= 1 {
			fmt.Fprintln(os.Stderr, "agon: unset stale ANTHROPIC_API_KEY for this run (claude OAuth keychain will be used)")
		}
	}

	// 4. v0 mode gates.
	if f.Main == "codex" {
		return nil, &PreflightError{Code: 102, Msg: "--main codex is v1; v0 supports --main claude only"}
	}
	if f.Judge != "none" {
		return nil, &PreflightError{Code: 103, Msg: "--judge llm/human is v1; v0 supports --judge none only"}
	}

	// 5. Family/model rule.
	if agentFamily(f.Main) == agentFamily(f.Side) {
		if f.MainModel == "" || f.SideModel == "" {
			return nil, &PreflightError{
				Code: 110,
				Msg:  "--main and --side are the same family; both --main-model and --side-model are required and must differ",
			}
		}
		if f.MainModel == f.SideModel {
			return nil, &PreflightError{
				Code: 111,
				Msg:  "--main-model and --side-model must differ when --main and --side are the same family",
			}
		}
	}

	// 6. Side-count.
	if f.SideCount < 1 {
		return nil, &PreflightError{Code: 121, Msg: "--side-count must be ≥ 1"}
	}
	if f.MaxTurn < 1 {
		return nil, &PreflightError{Code: 122, Msg: "--max-turn must be ≥ 1 (one critic↔proposer exchange minimum)"}
	}
	if f.CostCap < 1 {
		return nil, &PreflightError{Code: 123, Msg: "--cost-cap must be ≥ 1"}
	}
	if f.ChangedLinesMin < 0 {
		return nil, &PreflightError{Code: 124, Msg: "--changed-lines-min must be ≥ 0"}
	}
	if f.LogMode != "" && !IsValidLogMode(f.LogMode) {
		return nil, &PreflightError{
			Code: 125,
			Msg:  "--log-mode must be one of: silent, concise, verbose",
		}
	}

	// 7. Task-context source.
	if f.SessionID == "" && f.Transcript == "" && f.TaskContext == "" {
		return nil, &PreflightError{
			Code: 130,
			Msg:  "cannot determine task context: pass --session-id, --transcript, or --task-context",
		}
	}

	// 8. State dir writability.
	stateDir := f.StateDir
	if !filepath.IsAbs(stateDir) {
		stateDir = filepath.Join(cwd, stateDir)
	}
	parent := filepath.Dir(stateDir)
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return nil, &PreflightError{
			Code: 140,
			Msg:  fmt.Sprintf("cannot write under %s", parent),
			Wrap: err,
		}
	}

	// 9. .gitignore advisory - never an error.
	if missingFromGitignore(stateDir) {
		fmt.Fprintln(os.Stderr, "agon: warning: .agon/ is not in .gitignore - consider adding it before committing")
	}

	// Build forks plan.
	forks := make([]ForkPlan, f.SideCount)
	for i := 0; i < f.SideCount; i++ {
		forks[i] = ForkPlan{Index: i + 1}
	}

	return &Plan{
		Cwd:            cwd,
		Forks:          forks,
		StateDirAbs:    stateDir,
		SessionID:      f.SessionID,
		TranscriptPath: f.Transcript,
	}, nil
}

func agentFamily(name string) string {
	switch name {
	case "claude":
		return "claude"
	case "codex":
		return "codex"
	default:
		return name
	}
}

// encodedSegmentFromTranscript pulls the encoded cwd directory name
// embedded in a path like ~/.claude/projects/-Users-x-y/<id>.jsonl
// and returns it verbatim. Returns "" if the path doesn't match that
// shape. Compare the result against input.EncodeCwd(cwd) to check
// whether the transcript's session lives under the current working
// directory.
//
// The match requires the *.claude/projects/<encoded>* triple
// consecutively, not just any directory named "projects". An
// unrelated `projects` segment in a workspace path (e.g.
// /tmp/work/projects/notes/session.jsonl) used to false-flag and
// reject otherwise-valid transcripts (regression spotted by agon
// c1-1, 2026-05-07).
//
// Decoding the segment back to a path is intrinsically lossy (claude
// maps both `/` and `.` to `-`), so any check that needs to be sound
// must compare in encoded space.
func encodedSegmentFromTranscript(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(abs), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == ".claude" && parts[i+1] == "projects" {
			return parts[i+2]
		}
	}
	return ""
}

func missingFromGitignore(stateDir string) bool {
	root, err := gitRoot()
	if err != nil {
		return false
	}
	b, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, stateDir)
	if err != nil {
		return false
	}
	target := filepath.ToSlash(rel)
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimSuffix(line, "/")
		if line == target || line == target+"/" || strings.HasPrefix(target+"/", line+"/") {
			return false
		}
	}
	return true
}
