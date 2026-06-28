package topos_test

import (
	"context"
	"io"
	"testing"

	"latere.ai/x/agon/internal/critic"
	adversarial "latere.ai/x/agon/pkg/adversarial"
	atopos "latere.ai/x/agon/pkg/adversarial/topos"
	xtopos "latere.ai/x/topos"
	"latere.ai/x/topos/models"
)

// cannedR1 is a well-formed round-1 attack block (critic 1, aspect security,
// one attack) shaped like the contract in spec 13/14, so critic.Parse accepts
// it exactly as it would a claude or codex critic's output.
const cannedR1 = "# Critic 1 - round 1 attacks\n\n" +
	"aspect: security\n\n" +
	"## c1-1 [src/api.py:88]\n\n" +
	"claim: The search handler concatenates user input into a SQL LIKE pattern without escaping.\n\n" +
	"expected violation: An attacker can inject boolean logic via q=%' OR 1=1--.\n\n" +
	"reproduction:\n```\ncurl 'http://localhost:8000/search?q=1'\n```\n"

// scriptedBrain is a models.Model that emits a fixed text body then ends the
// turn, so a critic round is deterministic and network-free.
type scriptedBrain struct{ text string }

func (b scriptedBrain) Stream(_ context.Context, _ models.Request) (models.Stream, error) {
	return &scriptStream{events: []models.Event{
		{Kind: models.KindTextDelta, TextDelta: b.text},
		{Kind: models.KindDone, StopReason: models.StopEndTurn},
	}}, nil
}

type scriptStream struct {
	events []models.Event
	i      int
}

func (s *scriptStream) Recv() (models.Event, error) {
	if s.i >= len(s.events) {
		return models.Event{}, io.EOF
	}
	ev := s.events[s.i]
	s.i++
	return ev, nil
}

func (s *scriptStream) Close() error { return nil }

func runOnce(t *testing.T, cfg atopos.Config, in adversarial.CriticInput) *adversarial.CriticResult {
	t.Helper()
	res, err := atopos.NewCriticFactory(cfg)(1).Round(context.Background(), in)
	if err != nil {
		t.Fatalf("Round: %v", err)
	}
	return res
}

func securityInput() adversarial.CriticInput {
	return adversarial.CriticInput{
		AspectName:   "security",
		SystemPrompt: "you are a security critic",
		CriticIndex:  1,
		Round:        1,
		TaskContext:  "review the diff",
		DiffPatch:    "--- a/x\n+++ b/x\n@@\n-old\n+new\n",
	}
}

// TestFactoryYieldsCritic pins that NewCriticFactory returns an
// adversarial.Critic (compile-time + runtime).
func TestFactoryYieldsCritic(t *testing.T) {
	var _ adversarial.Critic = atopos.NewCriticFactory(atopos.Config{})(1)
}

// TestRoundReturnsModelTextVerbatim is the backend-swap contract: the topos
// critic returns the model's text unchanged as CriticResult.Markdown, and that
// markdown parses into the same Record set (critic.Parse) as the claude/codex
// critics on identical input.
func TestRoundReturnsModelTextVerbatim(t *testing.T) {
	res := runOnce(t, atopos.Config{Brain: scriptedBrain{text: cannedR1}}, securityInput())

	if res.Markdown != cannedR1 {
		t.Fatalf("markdown not verbatim:\n got %q\nwant %q", res.Markdown, cannedR1)
	}
	attacks, _, err := critic.Parse(res.Markdown, "security", 1, 1, nil, critic.ParseOption{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(attacks) != 1 {
		t.Fatalf("attacks: got %d, want 1", len(attacks))
	}
	if attacks[0].AttackID != "c1-1" {
		t.Errorf("attack id: got %q, want c1-1", attacks[0].AttackID)
	}
}

// TestRoundReportsNoUsage documents spec 39 OQ-3: topos's public RunResult
// exposes no token usage, so the topos critic reports zero. This pins the
// known limitation so a future topos-side fix flips this test deliberately.
func TestRoundReportsNoUsage(t *testing.T) {
	res := runOnce(t, atopos.Config{Brain: scriptedBrain{text: cannedR1}}, securityInput())
	if res.Usage.Total() != 0 || res.Tokens != 0 || res.USD != 0 {
		t.Errorf("expected zero usage (OQ-3), got usage=%+v tokens=%d usd=%v", res.Usage, res.Tokens, res.USD)
	}
}

// TestToposGrantsExactlyAgentSpecTools verifies the topos grant semantics the
// read-only posture relies on: an agent is granted exactly AgentSpec.Tools and
// nothing more, so nil tools => no grants => no bash, while an explicit bash
// grant does appear (control, so the nil assertion is not vacuous). The critic
// builds its AgentSpec with Tools=Config.Tools (nil default; see critic.go); that
// one-line wiring is not separately exercised here because CriticResult does not
// surface lineage (spec 39 OQ-2), so this asserts the property the wiring leans
// on, against the same public lineage Grants the critic would produce.
func TestToposGrantsExactlyAgentSpecTools(t *testing.T) {
	grants := func(tools []string) []string {
		runner, err := xtopos.NewRunner(xtopos.Options{SessionID: "t", Brain: scriptedBrain{text: "ok"}})
		if err != nil {
			t.Fatalf("NewRunner: %v", err)
		}
		region := xtopos.Region{
			Autonomy: xtopos.Pinned,
			Entry:    xtopos.AgentSpec{Name: "critic-1", Role: "critic", Tools: tools},
		}
		res, err := runner.Run(context.Background(), region, "prompt")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if len(res.Lineage.Nodes) == 0 {
			t.Fatal("no lineage nodes")
		}
		return res.Lineage.Nodes[0].Grants
	}

	// Default critic posture: nil tools => no grants => no bash.
	for _, g := range grants(nil) {
		t.Errorf("read-only critic was granted a tool: %q", g)
	}
	// Control: an explicit bash grant does show up, proving the assertion above
	// is meaningful rather than vacuous.
	var sawBash bool
	for _, g := range grants([]string{"bash"}) {
		if g == "bash" {
			sawBash = true
		}
	}
	if !sawBash {
		t.Error("control: expected bash in grants when explicitly granted")
	}
}
