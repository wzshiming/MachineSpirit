# MachineSpirit core functions (from OpenClaw)

Minimum pieces needed to realize the MachineSpirit:

- **Gateway (WS control plane)**: single process that terminates channel connectors, routes messages by session, streams tool invocations, and enforces auth.
- **Channels**: adapters for chat surfaces (WhatsApp/Telegram/Slack/Discord/etc.) with DM pairing or allowlists and basic chunking/retry.
- **Sessions & agents**: per-session agent runtime so conversations stay isolated; tracks presence/typing and queue/activation rules.
- **Tools/skills**: browser, exec, cron, canvas/visuals, and custom skills streamed over the Gateway with retry/failover.
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
