package agent

// AgentStrings contains all localizable strings for the agent package
type AgentStrings struct {
	AvailableSkillsHeader string
	UseSkillsHint         string
	AvailableToolsHeader  string
	ToolCallInstructions  string
	MultipleToolCallsHint string
	PreferSkillsHint      string
	SubSessionHint        string
	FinalResponsePrompt   string
	ReplanPrompt          string
}

// EnglishStrings returns English language strings
func EnglishStrings() AgentStrings {
	return AgentStrings{
		AvailableSkillsHeader: "## Available Skills:\n",
		UseSkillsHint:         "**IMPORTANT**: When you need to use any skill, you MUST first read the corresponding SKILL.md, then follow its instructions exactly. Do not guess or use skills from memory.\n",
		AvailableToolsHeader:  "## Available Tools:\n",
		ToolCallInstructions:  "To use a tool, respond with: <tool_call name=\"tool_name\">{\"param1\": \"value1\", \"param2\": \"value2\"}</tool_call>\n",
		MultipleToolCallsHint: "You can make multiple tool calls in a single response.\n",
		PreferSkillsHint:      "Prefer using skills for complex, multi-step operations when available.\n\n",
		SubSessionHint:        "Use the sub_session tool to delegate independent sub-tasks that can run in parallel, or for long-running operations that should not block the conversation.\n",
		FinalResponsePrompt:   "Based on these results, provide a final response to the user.\n",
		ReplanPrompt:          "\n\nSome tools failed. Please replan or provide an alternative solution. (Attempt %d/%d)",
	}
}

// ChineseStrings returns Chinese language strings
func ChineseStrings() AgentStrings {
	return AgentStrings{
		AvailableSkillsHeader: "## 可用 Skills:\n",
		UseSkillsHint:         "**重要**：当你需要使用任何技能时，必须先阅读对应的 SKILL.md 文件，并严格按照其中的说明操作。不要凭记忆猜测或使用技能。\n",
		AvailableToolsHeader:  "## 可用 Tools:\n",
		ToolCallInstructions:  "要使用工具，请回复：<tool_call name=\"tool_name\">{\"param1\": \"value1\", \"param2\": \"value2\"}</tool_call>\n",
		MultipleToolCallsHint: "你可以在一次响应中进行多次工具调用。\n",
		PreferSkillsHint:      "在可用时优先使用技能进行复杂的多步操作。\n\n",
		SubSessionHint:        "使用 sub_session 工具将独立的子任务委托到后台并行执行，或用于不应阻塞对话的长时间运行操作。\n",
		FinalResponsePrompt:   "基于这些结果，向用户提供最终响应。\n",
		ReplanPrompt:          "\n\n一些工具失败了。请重新规划或提供替代方案。（尝试 %d/%d）",
	}
}

// GetStrings returns the appropriate strings based on locale
func GetStrings(locale string) AgentStrings {
	switch locale {
	case "zh":
		return ChineseStrings()
	default:
		return EnglishStrings()
	}
}
