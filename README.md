# MachineSpirit

MachineSpirit is a lightweight, hackable CLI framework for building AI agents.

It combines model providers, tool execution, skill discovery, and editable prompt files into a small, composable runtime that works well for both interactive use and scripting.

## Highlights

- Interactive REPL plus stdin pipe mode for automation workflows
- Pluggable provider support for OpenAI and Anthropic
- Built-in filesystem and shell tools: `bash`, `read`, `write`
- Skill discovery via `SKILL.md` conventions
- Editable workspace prompt files to shape behavior and memory

## Workspace Files

Auto-initialized when missing:

- `SOUL.md`
- `AGENTS.md`
- `IDENTITY.md`
- `USER.md`
- `TOOLS.md`
- `BOOTSTRAP.md`

Also reads `MEMORY.md` if present.

## Skills

Scans `SKILL.md` files from:

- `$HOME/.agents/skills`
- `.agents/skills`

## Inspiration

- [OpenClaw](https://github.com/openclaw/openclaw): Your own personal AI assistant.
- [Machine Spirit](https://warhammer40k.fandom.com/wiki/Machine_Spirit) ([Warhammer 40K](https://en.wikipedia.org/wiki/Warhammer_40,000)): People believe important machines have a spirit. If you do not care for them, they can stop working.
- [The Omnissiah](https://warhammer40k.fandom.com/wiki/Omnissiah) ([Warhammer 40K](https://en.wikipedia.org/wiki/Warhammer_40,000)): The machine god in Adeptus Mechanicus belief.

## License

Licensed under the MIT License. See [LICENSE](https://github.com/wzshiming/MachineSpirit/blob/master/LICENSE) for the full license text.
