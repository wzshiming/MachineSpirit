package agent

import (
	"fmt"
	"sync"
)

// SkillRegistry manages the collection of available skills.
type SkillRegistry struct {
	mu     sync.RWMutex
	skills map[string]Skill
}

// NewSkillRegistry creates a new skill registry.
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{
		skills: make(map[string]Skill),
	}
}

// Register adds a skill to the registry.
func (r *SkillRegistry) Register(skill Skill) error {
	if skill == nil {
		return fmt.Errorf("skill cannot be nil")
	}
	name := skill.Name()
	if name == "" {
		return fmt.Errorf("skill name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.skills[name]; exists {
		return fmt.Errorf("skill %q already registered", name)
	}

	r.skills[name] = skill
	return nil
}

// Unregister removes a skill from the registry.
func (r *SkillRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.skills[name]; !exists {
		return fmt.Errorf("skill %q not found", name)
	}

	delete(r.skills, name)
	return nil
}

// Get retrieves a skill by name.
func (r *SkillRegistry) Get(name string) (Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, exists := r.skills[name]
	if !exists {
		return nil, fmt.Errorf("skill %q not found", name)
	}

	return skill, nil
}

// List returns all registered skills.
func (r *SkillRegistry) List() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skills := make([]Skill, 0, len(r.skills))
	for _, skill := range r.skills {
		skills = append(skills, skill)
	}
	return skills
}

// Has checks if a skill is registered.
func (r *SkillRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.skills[name]
	return exists
}

// Count returns the number of registered skills.
func (r *SkillRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.skills)
}

// Clear removes all skills from the registry.
func (r *SkillRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skills = make(map[string]Skill)
}
