package agent

// Skill represents a high-level, composable capability that an agent can use.
type Skill interface {
	// Name returns the unique identifier for this skill.
	Name() string
	// Description returns a brief, human-readable explanation of what this skill does.
	Description() string
	// Path returns the file path of the skill definition (if applicable).
	Path() string
}
