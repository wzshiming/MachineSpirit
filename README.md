# MachineSpirit Architecture (derived from OpenClaw)

This repository captures the core design patterns of [OpenClaw](https://github.com/openclaw/openclaw) so they can be reused by MachineSpirit. The focus is on the architectural shape of the system rather than implementation details.

## Goals and design principles

- **Local-first control plane**: a single Gateway process manages sessions, tools, and events over a WebSocket network.
- **Multi-channel inbox**: connect many chat surfaces (WhatsApp, Telegram, Slack, Discord, Google Chat, Signal, iMessage/BlueBubbles, IRC, Microsoft Teams, Matrix, Feishu, LINE, Mattermost, Nextcloud Talk, Nostr, Synology Chat, Tlon, Twitch, Zalo/Zalo Personal, WebChat) and route them safely.
- **Agent-per-session runtime**: isolate state per session with routing, activation, and reply rules to avoid cross-talk.
- **Tool-first orchestration**: rich tools (browser, cron, exec, canvas, nodes) are first-class and streamed over the Gateway.
- **Safety by default**: DM pairing/allowlists, retry/backoff, model failover, permission-aware device actions.

## High-level topology

```
Inbound channels ─▶ Gateway (WS control plane) ─▶ Agents/CLI/WebChat
                                         └────▶ Devices as nodes (macOS/iOS/Android)
                                         └────▶ Tools (browser, exec, cron, canvas)
```

## Core components

- **Gateway**: WebSocket control plane that terminates channel connectors, tracks presence, manages sessions, routes messages, streams tool calls, enforces auth, and exposes a control UI + WebChat.
- **Channels**: adapters for the supported messaging platforms with DM policies (pairing/open), allowlists, chunking, and retries. Group routing keeps replies scoped.
- **Agents (Pi runtime)**: per-session agents with streaming tool calls, model failover/rotation, session pruning, and queue/activation modes.
- **Tools/skills**: browser automation, exec, cron, canvas (A2UI), plus custom skills; all use the Gateway tool protocol for streaming and retries.
- **Nodes**: device endpoints (macOS/iOS/Android) advertising capabilities via `node.list`/`node.describe`; actions go through `node.invoke` (e.g., `system.run`, `system.notify`, camera, screen recording, location).
- **Surfaces**: CLI (`openclaw ...`), WebChat, macOS menu bar app, Android/iOS apps—all connect via the Gateway WebSocket.

## Message and tool flow

1. A channel connector receives an inbound message and applies DM/group rules.
2. Gateway routes the event into the correct session (agent instance) and updates presence/typing indicators.
3. The agent processes the message, streaming tool calls (browser/exec/skills/nodes) as needed.
4. Responses are streamed back through the Gateway and delivered over the originating channel with chunking/retry.
5. Logs and transcripts remain session-scoped; doctor/runbooks surface misconfigurations.

## Reliability and safety features

- Channel-aware chunking, retries, and backoff.
- Model failover with preferred/backup providers and auth profile rotation.
- Session pruning and queue/activation controls to prevent overload.
- Permission-aware node actions (TCC on macOS/iOS; explicit elevated toggle for host exec).
- Optional Tailscale Serve/Funnel or SSH tunnels for remote Gateway access while keeping loopback binding.

## Deployment notes (for MachineSpirit)

- Runtime target: Node ≥22; prefer `pnpm` for builds.
- Gateway should run as a user daemon/service for always-on behavior; clients connect via `ws://127.0.0.1:18789` (or tunneled).
- Default posture: keep gateway bind on loopback, enable DM pairing, and restrict allowlists; turn on doctor checks early.
- Remote setups split responsibilities: Gateway host runs exec/tooling; device nodes handle local-only actions and permissions.

## Next steps for this repository

- Map MachineSpirit requirements onto this architecture (channels to support, tool set, security posture).
- Prototype the Gateway control plane and session model first; add channels and nodes incrementally.
- Keep documentation updated as components are implemented. 
