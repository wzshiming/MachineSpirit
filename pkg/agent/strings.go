package agent

// agentStrings contains all localizable strings for the agent package
type agentStrings struct {
	AvailableSkillsHeader string
	UseSkillsHint         string
	AvailableToolsHeader  string
	ToolCallInstructions  string
	MultipleToolCallsHint string
	PreferSkillsHint      string
	SubSessionHint        string
	CompressHint          string
	FinalResponsePrompt   string
	ReplanPrompt          string
}

// englishStrings returns English language strings
func englishStrings() agentStrings {
	return agentStrings{
		AvailableSkillsHeader: "## Available Skills:\n",
		UseSkillsHint:         "**IMPORTANT**: When you need to use any skill, you MUST first read the corresponding SKILL.md, then follow its instructions exactly. Do not guess or use skills from memory.\n",
		AvailableToolsHeader:  "## Available Tools:\n",
		ToolCallInstructions:  "To use a tool, respond with: <tool_call name=\"tool_name\">{\"param1\": \"value1\", \"param2\": \"value2\"}</tool_call>\n",
		MultipleToolCallsHint: "You can make multiple tool calls in a single response.\n",
		PreferSkillsHint:      "Prefer using skills for complex, multi-step operations when available.\n\n",
		SubSessionHint:        "Use the sub_session tool to delegate independent sub-tasks that can run in parallel, or for long-running operations that should not block the conversation.\n",
		CompressHint:          "**IMPORTANT**: Monitor the transcript size reported by the compress_transcript tool. When the transcript exceeds the threshold, you MUST call compress_transcript to free up context window space. Do not wait for the user to ask.\n",
		FinalResponsePrompt:   "Based on these results, provide a final response to the user.\n",
		ReplanPrompt:          "\n\nSome tools failed. Please replan or provide an alternative solution. (Attempt %d/%d)",
	}
}
