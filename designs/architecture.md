# MachineSpirit core functions (from OpenClaw)

Core implementation (first step):

- **Sessions & agents**: per-session runtime that owns conversation state. Handles intake from Gateway, applies queue/activation rules, updates presence/typing, and drives the agent loop (plan → tool calls → replies).
  - Message path: channel event → Gateway session inbox → agent state update → response envelope (with typing/presence) → Gateway egress.
  - State hygiene: prune inactive sessions; keep transcripts scoped per session.

  Minimal program:
  1) Session creation/lookup on inbound event.
  2) Apply queue/activation rules; drop/park if session is inactive/overloaded.
  3) Update presence/typing; run agent loop (plan → tool calls → compose reply).
  4) Emit response envelope; update transcript; schedule pruning timers.

Core implementation (second step):

- **Tools/skills**: orchestrator for browser, exec, cron, canvas/visuals, and custom skills. Stream calls over the Gateway with retry/failover and chunked outputs.
  - Tool path: agent invokes tool → streamed over Gateway → tool runtime executes (may be remote) → partials/results streamed back → agent composes reply.
  - Failovers: retry on transient errors; allow model/tool fallback when declared.

  Minimal program:
  1) Define tool contracts + timeouts; mark which can stream partials.
  2) Stream invocation over Gateway; track call id.
  3) Collect partials/finals; on retryable error, retry with backoff or fallback.
  4) Return structured result to agent for reply composition.

Peripheral packaging:

- **Gateway (WS control plane)**: single process that terminates channel connectors, routes messages by session, streams tool invocations, and enforces auth.
- **Channels**: adapters for chat surfaces (WhatsApp/Telegram/Slack/Discord/etc.) with DM pairing or allowlists and basic chunking/retry.
- **Nodes (devices)**: macOS/iOS/Android endpoints advertising capabilities; invoked via `node.invoke` for local actions like `system.run` or camera/screen tasks.
- **Surfaces**: CLI/WebChat/apps connect via the Gateway WebSocket; no direct channel coupling.

Flow (simplified):

```
Inbound channel ─▶ Gateway ─▶ Session agent ─▶ Tools / Nodes ─▶ Gateway ─▶ Outbound channel
```

Reliability & safety (minimal set):

- Channel-aware chunking + retry/backoff.
- Session pruning and basic queue controls.
- Permission-aware node actions; DM pairing by default. 

Differences from OpenClaw:

- Scope is reduced to the two core steps (sessions/agents, tools/skills); Gateway/channels/nodes/surfaces are treated as packaging, not expansion targets.
- No multi-channel breadth assumed up front; start with the minimum channels/nodes needed, then grow.
- Tooling is focused on orchestrator patterns (streamed calls, retries/fallbacks) rather than specific skill catalogs. 
