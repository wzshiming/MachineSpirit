package skills

import (
	"strings"
)

// Registry stores available skills by name.
type Registry struct {
	skills map[string]Skill
}

// NewRegistry constructs a registry from the provided skills.
func NewRegistry(list ...Skill) Registry {
	r := Registry{
		skills: make(map[string]Skill, len(list)),
	}
	for _, s := range list {
		if s == nil || s.Name() == "" {
			continue
		}
		r.skills[strings.ToLower(s.Name())] = s
	}
	return r
}

// Get returns a skill by exact name (case-insensitive).
func (r Registry) Get(name string) (Skill, bool) {
	if name == "" {
		return nil, false
	}
	s, ok := r.skills[strings.ToLower(name)]
	return s, ok
}

// List returns all registered skills.
func (r Registry) List() []Skill {
	out := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	return out
}

// Selector chooses a skill given a requested name or intent text.
type Selector struct {
	Registry Registry
}

// Select returns the best skill match for the provided token. Prefers exact name,
// falls back to substring matches over name then description.
func (s Selector) Select(token string) (Skill, bool) {
	if s.Registry.skills == nil {
		return nil, false
	}

	if exact, ok := s.Registry.Get(token); ok {
		return exact, true
	}

	tokenLower := strings.ToLower(strings.TrimSpace(token))
	if tokenLower == "" {
		return nil, false
	}

	var candidate Skill
	for _, skill := range s.Registry.skills {
		if strings.Contains(strings.ToLower(skill.Name()), tokenLower) {
			return skill, true
		}
		if candidate == nil && strings.Contains(strings.ToLower(skill.Description()), tokenLower) {
			candidate = skill
		}
	}
	if candidate != nil {
		return candidate, true
	}
	return nil, false
}
