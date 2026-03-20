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
		AvailableSkillsHeader: "\n<skills>\n",
		UseSkillsHint:         "Use skills for complex, multi-step operations when available.\n</skills>\n\n",
		AvailableToolsHeader:  "<tools>\n",
		ToolCallInstructions:  "</tools>\n\n<instructions>\nTo call a tool, respond with:\n<tool_call name=\"tool_name\">{\"param1\": \"value1\", \"param2\": \"value2\"}</tool_call>\n",
		MultipleToolCallsHint: "You can make multiple tool calls in a single response.\n",
		PreferSkillsHint:      "Prefer using skills for complex, multi-step operations when available.\n</instructions>\n\n",
		SubSessionHint:        "Use the sub_session tool to delegate independent sub-tasks that can run in parallel, or for long-running operations that should not block the conversation.\n",
		FinalResponsePrompt:   "Based on these results, provide a final response to the user.\n",
		ReplanPrompt:          "\n\nSome tools failed. Please replan or provide an alternative solution. (Attempt %d/%d)",
	}
}

// ChineseStrings returns Chinese language strings
func ChineseStrings() AgentStrings {
	return AgentStrings{
		AvailableSkillsHeader: "\n<skills>\n",
		UseSkillsHint:         "在可用时使用技能进行复杂的多步操作。\n</skills>\n\n",
		AvailableToolsHeader:  "<tools>\n",
		ToolCallInstructions:  "</tools>\n\n<instructions>\n要使用工具，请回复：\n<tool_call name=\"tool_name\">{\"param1\": \"value1\", \"param2\": \"value2\"}</tool_call>\n",
		MultipleToolCallsHint: "你可以在一次响应中进行多次工具调用。\n",
		PreferSkillsHint:      "在可用时优先使用技能进行复杂的多步操作。\n</instructions>\n\n",
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
