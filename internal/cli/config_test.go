package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestProjectConfigOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".agon.toml")
	if err := os.WriteFile(cfg, []byte(`max_turn = 10
side_count = 2
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{Use: "agon"}
	f := Bind(cmd)
	f.Config = cfg
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Effective(cmd, f); err != nil {
		t.Fatal(err)
	}
	if f.MaxTurn != 10 {
		t.Errorf("max_turn from file: got %d, want 10", f.MaxTurn)
	}
	if f.SideCount != 2 {
		t.Errorf("side_count from file: got %d, want 2", f.SideCount)
	}
}

// TestAllowStyleAttacksFromConfig proves the TOML allow_style_attacks
// key is wired through to Flags. Before the fix it parsed but was never
// read, so setting it was a silent no-op.
func TestAllowStyleAttacksFromConfig(t *testing.T) {
	t.Run("true flips the flag", func(t *testing.T) {
		dir := t.TempDir()
		cfg := filepath.Join(dir, ".agon.toml")
		if err := os.WriteFile(cfg, []byte("allow_style_attacks = true\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := &cobra.Command{Use: "agon"}
		f := Bind(cmd)
		f.Config = cfg
		if err := cmd.ParseFlags(nil); err != nil {
			t.Fatal(err)
		}
		if _, err := Effective(cmd, f); err != nil {
			t.Fatal(err)
		}
		if !f.AllowStyleAttacks {
			t.Error("allow_style_attacks=true did not set f.AllowStyleAttacks")
		}
	})
	t.Run("unset leaves the default false", func(t *testing.T) {
		dir := t.TempDir()
		cfg := filepath.Join(dir, ".agon.toml")
		if err := os.WriteFile(cfg, []byte("max_turn = 4\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := &cobra.Command{Use: "agon"}
		f := Bind(cmd)
		f.Config = cfg
		if err := cmd.ParseFlags(nil); err != nil {
			t.Fatal(err)
		}
		if _, err := Effective(cmd, f); err != nil {
			t.Fatal(err)
		}
		if f.AllowStyleAttacks {
			t.Error("allow_style_attacks unset should leave f.AllowStyleAttacks false")
		}
	})
}

func TestCLIFlagWinsOverConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".agon.toml")
	if err := os.WriteFile(cfg, []byte(`max_turn = 10`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{Use: "agon"}
	f := Bind(cmd)
	f.Config = cfg
	if err := cmd.ParseFlags([]string{"--max-turn", "3"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Effective(cmd, f); err != nil {
		t.Fatal(err)
	}
	if f.MaxTurn != 3 {
		t.Errorf("CLI flag should win: got %d, want 3", f.MaxTurn)
	}
}

func TestUnknownKeyRejected(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".agon.toml")
	if err := os.WriteFile(cfg, []byte(`bogus_key = 42`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{Use: "agon"}
	f := Bind(cmd)
	f.Config = cfg
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Effective(cmd, f); err == nil {
		t.Error("expected error for unknown TOML key")
	}
}

// TestExplicitMissingConfigErrors pins that an explicit --config path
// that does not exist fails with ErrConfigNotFound rather than silently
// running on defaults. Auto-discovery (empty Config) stays non-fatal,
// covered by TestEffective_NoConfigPaths.
func TestExplicitMissingConfigErrors(t *testing.T) {
	cmd := &cobra.Command{Use: "agon"}
	f := Bind(cmd)
	f.Config = filepath.Join(t.TempDir(), "does-not-exist.toml")
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	_, err := Effective(cmd, f)
	if err == nil {
		t.Fatal("expected error for missing explicit --config path, got nil")
	}
	if !errors.Is(err, ErrConfigNotFound) {
		t.Errorf("error: got %v, want wrap of ErrConfigNotFound", err)
	}
}

func TestUserConfigPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/x")
	if got := userConfigPath(); got != filepath.Join("/x", "agon", "config.toml") {
		t.Errorf("XDG branch: got %q", got)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/h")
	if got := userConfigPath(); got != filepath.Join("/h", ".config", "agon", "config.toml") {
		t.Errorf("HOME branch: got %q", got)
	}
}

func TestProjectConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Explicit wins.
	if got := projectConfigPath("/explicit/path"); got != "/explicit/path" {
		t.Errorf("explicit: got %q", got)
	}

	// No file in cwd, no git root: empty.
	if got := projectConfigPath(""); got != "" {
		t.Errorf("no candidate: got %q", got)
	}

	// File in cwd: returned.
	cfg := filepath.Join(dir, ".agon.toml")
	if err := os.WriteFile(cfg, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got := projectConfigPath("")
	gotResolved, _ := filepath.EvalSymlinks(got)
	cfgResolved, _ := filepath.EvalSymlinks(cfg)
	if gotResolved != cfgResolved {
		t.Errorf("cwd file branch: got %q, want %q", got, cfg)
	}
}

func TestApplyConfigToFlags_AllKeys(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".agon.toml")
	body := `main = "claude"
side = "codex"
side_count = 3
main_model = "m1"
side_model = "m2"
max_turn = 9
cost_cap_tokens = 12345
changed_lines_min = 5
state_dir = "/tmp/x"
format = "markdown"
judge = "none"
trigger = "manual"
allow_style_attacks = true
`
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{Use: "agon"}
	f := Bind(cmd)
	f.Config = cfg
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Effective(cmd, f); err != nil {
		t.Fatal(err)
	}
	cases := map[string]any{
		"Main":            f.Main,
		"Side":            f.Side,
		"SideCount":       f.SideCount,
		"MainModel":       f.MainModel,
		"SideModel":       f.SideModel,
		"MaxTurn":         f.MaxTurn,
		"CostCap":         f.CostCap,
		"ChangedLinesMin": f.ChangedLinesMin,
		"StateDir":        f.StateDir,
		"Format":          f.Format,
		"Judge":           f.Judge,
	}
	if cases["Main"] != "claude" {
		t.Errorf("Main: %v", cases["Main"])
	}
	if cases["Side"] != "codex" {
		t.Errorf("Side: %v", cases["Side"])
	}
	if cases["SideCount"] != 3 {
		t.Errorf("SideCount: %v", cases["SideCount"])
	}
	if cases["MainModel"] != "m1" {
		t.Errorf("MainModel: %v", cases["MainModel"])
	}
	if cases["SideModel"] != "m2" {
		t.Errorf("SideModel: %v", cases["SideModel"])
	}
	if cases["MaxTurn"] != 9 {
		t.Errorf("MaxTurn: %v", cases["MaxTurn"])
	}
	if cases["CostCap"] != 12345 {
		t.Errorf("CostCap: %v", cases["CostCap"])
	}
}

func TestEffective_NoConfigPaths(t *testing.T) {
	// Neither user nor project config exist; layering should still
	// produce a Flags struct with defaults applied.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	cmd := &cobra.Command{Use: "agon"}
	f := Bind(cmd)
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	out, err := Effective(cmd, f)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil {
		t.Fatal("nil flags returned")
	}
}
