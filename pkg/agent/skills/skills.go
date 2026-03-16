package skills

import (
	"log/slog"
)

type Skills struct {
	dirs []string
}

func NewSkills(dirs ...string) *Skills {
	return &Skills{dirs: dirs}
}

func (s *Skills) List() (list []*Skill) {
	for _, dir := range s.dirs {
		loader := newSkillLoader(dir)
		items, err := loader.LoadAllSkills()
		if err != nil {
			slog.Warn("Failed to load skills from directory", "dir", dir, "error", err)
			continue
		}
		list = append(list, items...)
	}
	return list
}
