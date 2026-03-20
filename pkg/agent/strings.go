package agent

// AgentStrings contains all localizable strings for the agent package
type AgentStrings struct {
	AvailableSkillsHeader string
	UseSkillsHint         string
	AvailableToolsHeader  string
	ToolCallInstructions  string
	MultipleToolCallsHint string
	PreferSkillsHint      string
	FinalResponsePrompt   string
	ReplanPrompt          string
}

// EnglishStrings returns English language strings
func EnglishStrings() AgentStrings {
	return AgentStrings{
		AvailableSkillsHeader: "## Available Skills (High-level capabilities):\n",
		UseSkillsHint:         "Use skills for complex, multi-step operations when available.\n\n",
		AvailableToolsHeader:  "## Available Tools (Low-level operations):\n",
		ToolCallInstructions:  "To use a tool, respond with: <tool_call name=\"tool_name\">{...}</tool_call>\n",
		MultipleToolCallsHint: "You can make multiple tool calls in a single response.\n",
		PreferSkillsHint:      "Prefer using skills for complex, multi-step operations when available.\n\n",
		FinalResponsePrompt:   "Based on these results, provide a final response to the user.\n",
		ReplanPrompt:          "\n\nSome tools failed. Please replan or provide an alternative solution. (Attempt %d/%d)",
	}
}

// ChineseStrings returns Chinese language strings
func ChineseStrings() AgentStrings {
	return AgentStrings{
		AvailableSkillsHeader: "## 可用技能（高级功能）：\n",
		UseSkillsHint:         "在可用时使用技能进行复杂的多步操作。\n\n",
		AvailableToolsHeader:  "## 可用工具（底层操作）：\n",
		ToolCallInstructions:  "要使用工具，请回复：<tool_call name=\"tool_name\">{...}</tool_call>\n",
		MultipleToolCallsHint: "你可以在一次响应中进行多次工具调用。\n",
		PreferSkillsHint:      "在可用时优先使用技能进行复杂的多步操作。\n\n",
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
