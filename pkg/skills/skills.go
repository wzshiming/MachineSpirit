package skills

import "context"

// Skill represents a callable capability with metadata useful for selection.
type Skill interface {
	Name() string
	Description() string
	Invoke(ctx context.Context, payload map[string]any) (string, error)
}

// Func adapts a function into a Skill with name/description metadata.
type Func struct {
	SkillName string
	Detail    string
	Handler   func(ctx context.Context, payload map[string]any) (string, error)
}

func (f Func) Name() string {
	return f.SkillName
}

func (f Func) Description() string {
	return f.Detail
}

func (f Func) Invoke(ctx context.Context, payload map[string]any) (string, error) {
	if f.Handler == nil {
		return "", nil
	}
	return f.Handler(ctx, payload)
}
