package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

// ErrConfigNotFound is returned when --config <path> points at a file
// that does not exist.
var ErrConfigNotFound = errors.New("config file not found")

// configFile mirrors .agon.toml's schema.
type configFile struct {
	Main              string `toml:"main"`
	Side              string `toml:"side"`
	SideCount         int    `toml:"side_count"`
	MainModel         string `toml:"main_model"`
	SideModel         string `toml:"side_model"`
	MaxTurn           int    `toml:"max_turn"`
	CostCapTokens     int    `toml:"cost_cap_tokens"`
	ChangedLinesMin   int    `toml:"changed_lines_min"`
	StateDir          string `toml:"state_dir"`
	Format            string `toml:"format"`
	Judge             string `toml:"judge"`
	Trigger           string `toml:"trigger"`
	AllowStyleAttacks bool   `toml:"allow_style_attacks"`

	// Tracked at decode time; non-nil = field present in file.
	present map[string]bool
}

// Effective returns the layered config: defaults → user config →
// project config → env → CLI flags. cmd's flag.Changed bit is the
// authoritative signal for "user supplied".
func Effective(cmd *cobra.Command, f *Flags) (*Flags, error) {
	user, err := loadConfig(userConfigPath())
	if err != nil {
		return nil, err
	}
	// An explicitly requested --config path must exist: silently falling
	// back to defaults hides a typo. Auto-discovered paths (empty
	// f.Config) stay non-fatal when absent.
	if f.Config != "" {
		if _, serr := os.Stat(f.Config); serr != nil {
			if errors.Is(serr, os.ErrNotExist) {
				return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, f.Config)
			}
			return nil, serr
		}
	}
	proj, err := loadConfig(projectConfigPath(f.Config))
	if err != nil {
		return nil, err
	}

	// Apply user then project: project wins on overlapping keys.
	for _, c := range []*configFile{user, proj} {
		if c == nil {
			continue
		}
		applyConfigToFlags(cmd, f, c)
	}

	// Env vars and CLI flags are layered by Bind+ApplyEnv elsewhere.
	ApplyEnv(cmd, f)

	return f, nil
}

func userConfigPath() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "agon", "config.toml")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "agon", "config.toml")
	}
	return ""
}

func projectConfigPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	cwd, err := os.Getwd()
	if err == nil {
		p := filepath.Join(cwd, ".agon.toml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if root, err := gitRoot(); err == nil {
		p := filepath.Join(root, ".agon.toml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func gitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func loadConfig(path string) (*configFile, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var c configFile
	c.present = map[string]bool{}
	md, err := toml.Decode(string(b), &c)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrConfigDecodeWrap(path), err.Error())
	}
	for _, k := range md.Keys() {
		c.present[strings.Join(k, ".")] = true
	}
	if undec := md.Undecoded(); len(undec) > 0 {
		keys := make([]string, len(undec))
		for i, k := range undec {
			keys[i] = strings.Join(k, ".")
		}
		return nil, fmt.Errorf("%w in %s: unknown key(s): %s",
			ErrConfigDecodeWrap(path), path, strings.Join(keys, ", "))
	}
	return &c, nil
}

// ErrConfigDecode is the sentinel error wrapping a path-aware decode
// failure; callers can errors.Is against it.
var ErrConfigDecode = errors.New("config decode error")

// ErrConfigDecodeWrap returns ErrConfigDecode; the path argument is
// retained for future structured wrapping.
func ErrConfigDecodeWrap(_ string) error { return ErrConfigDecode }

func applyConfigToFlags(cmd *cobra.Command, f *Flags, c *configFile) {
	set := func(flagName string, apply func()) {
		if cmd.Flags().Changed(flagName) {
			return
		}
		apply()
	}
	if c.present["main"] {
		set("main", func() { f.Main = c.Main })
	}
	if c.present["side"] {
		set("side", func() { f.Side = c.Side })
	}
	if c.present["side_count"] {
		set("side-count", func() { f.SideCount = c.SideCount })
	}
	if c.present["main_model"] {
		set("main-model", func() { f.MainModel = c.MainModel })
	}
	if c.present["side_model"] {
		set("side-model", func() { f.SideModel = c.SideModel })
	}
	if c.present["max_turn"] {
		set("max-turn", func() { f.MaxTurn = c.MaxTurn })
	}
	if c.present["cost_cap_tokens"] {
		set("cost-cap", func() { f.CostCap = c.CostCapTokens })
	}
	if c.present["changed_lines_min"] {
		set("changed-lines-min", func() { f.ChangedLinesMin = c.ChangedLinesMin })
	}
	if c.present["state_dir"] {
		set("state-dir", func() { f.StateDir = c.StateDir })
	}
	if c.present["format"] {
		set("format", func() { f.Format = c.Format })
	}
	if c.present["judge"] {
		set("judge", func() { f.Judge = c.Judge })
	}
	// trigger and allow_style_attacks have no CLI counterpart; stored
	// elsewhere or consulted directly from the configFile by future
	// specs (e.g., spec 14 reads allow_style_attacks).
}
