package skills

import (
	"context"
	"testing"
)

func TestSelectorPrefersExactName(t *testing.T) {
	reg := NewRegistry(
		Func{SkillName: "search", Detail: "web search", Handler: func(ctx context.Context, payload map[string]any) (string, error) {
			return "search", nil
		}},
		Func{SkillName: "weather", Detail: "get weather", Handler: func(ctx context.Context, payload map[string]any) (string, error) {
			return "weather", nil
		}},
	)
	selector := Selector{Registry: reg}

	skill, ok := selector.Select("weather")
	if !ok || skill.Name() != "weather" {
		t.Fatalf("expected weather skill, got %+v", skill)
	}
}

func TestSelectorFallsBackToDescription(t *testing.T) {
	reg := NewRegistry(
		Func{SkillName: "lookup", Detail: "retrieve documents", Handler: nil},
	)
	selector := Selector{Registry: reg}

	skill, ok := selector.Select("documents")
	if !ok || skill.Name() != "lookup" {
		t.Fatalf("expected lookup via description match, got %+v", skill)
	}
}

func TestInvokerResolvesAndRunsSkill(t *testing.T) {
	reg := NewRegistry(
		Func{SkillName: "echo", Handler: func(ctx context.Context, payload map[string]any) (string, error) {
			if payload == nil {
				return "", nil
			}
			if v, ok := payload["text"].(string); ok {
				return v, nil
			}
			return "", nil
		}},
	)
	inv := Invoker{Selector: Selector{Registry: reg}}

	out, err := inv.Invoke(context.Background(), "echo", map[string]any{"text": "hello"})
	if err != nil {
		t.Fatalf("invoke returned error: %v", err)
	}
	if out != "hello" {
		t.Fatalf("unexpected output: %q", out)
	}
}
