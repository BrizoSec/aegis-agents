# aegis-agents

The **Agents Component** of [Aegis OS](https://github.com/your-org/aegis-os) — a distributed operating system purpose-built for running autonomous AI agents.

---

## Overview

`aegis-agents` is an agent lifecycle management system. It acts as an agent factory: receiving task specifications from the Orchestrator, provisioning the right agent for each task (reusing existing agents or building new ones), and managing agents through their full lifecycle — from spawn to termination.

This component is one of five in the Aegis OS platform. It does not own task routing, persistent storage, secrets, or message transport — those belong to adjacent components. It integrates with all four through well-defined contracts.

---

## Responsibilities

- **Agent Provisioning** — Spawn new agents inside Firecracker microVMs when no capable agent exists for a task
- **Agent Registry** — Maintain a catalog of all agents, their capabilities, states, and credential permission sets
- **Skill Management** — Serve agent skills via a progressive disclosure hierarchy (domain → command → parameter spec)
- **Credential Brokering** — Pre-authorize credential access at spawn; deliver credentials lazily at runtime via OpenBao
- **Lifecycle Management** — Health monitoring, crash recovery, graceful shutdown, and VM teardown
- **State Persistence** — Delegate all persistence to the Memory Component via a disciplined interface

---

## Architecture

The component is organized into seven modules with a strict single-responsibility principle. All external communication flows through a single gateway — no module reaches out to an external component directly.

```
┌─────────────────────────────────────────────────────┐
│                  aegis-agents                        │
│                                                      │
│  ┌─────────────┐        ┌──────────────────────┐    │
│  │ Comms       │◄──────►│   Agent Factory (M2) │    │
│  │ Interface   │        └──────────────────────┘    │
│  │ (M1)        │           │        │        │      │
│  └─────────────┘           ▼        ▼        ▼      │
│         │           ┌────────┐ ┌──────┐ ┌────────┐  │
│         │           │Registry│ │Skills│ │Creds   │  │
│         │           │ (M3)   │ │ (M4) │ │Broker  │  │
│         │           └────────┘ └──────┘ │ (M5)   │  │
│         │                               └────────┘  │
│         │           ┌───────────────────────────┐   │
│         │           │  Lifecycle Manager (M6)   │   │
│         │           └───────────────────────────┘   │
│         │           ┌───────────────────────────┐   │
│         │           │  Memory Interface (M7)    │   │
│         │           └───────────────────────────┘   │
└─────────────────────────────────────────────────────┘
         │
         ▼
   ┌─────────────────────────────────┐
   │ External Components             │
   │  Orchestrator  │  Comms (NATS)  │
   │  OpenBao Vault │  Memory        │
   └─────────────────────────────────┘
```

| Module | Package | Role |
|--------|---------|------|
| M1 — Communications Interface | `internal/comms` | Single NATS gateway for all inter-component messaging |
| M2 — Agent Factory | `internal/factory` | Central coordinator for all agent provisioning |
| M3 — Agent Registry | `internal/registry` | In-memory catalog backed by Memory Component |
| M4 — Skill Hierarchy Manager | `internal/skills` | Three-level skill tree with on-demand discovery |
| M5 — Credential Broker | `internal/credentials` | Two-phase credential authorization via OpenBao |
| M6 — Lifecycle Manager | `internal/lifecycle` | Firecracker microVM spawn, monitoring, teardown |
| M7 — Memory Interface | `internal/memory` | Disciplined persistence layer to Memory Component |

---

## Key Design Decisions

**Progressive Skill Disclosure** — Agents do not receive their full skill set at spawn. Skills are served on demand as agents drill down the hierarchy. This prevents context rot and keeps agent context focused on the active task.

**Lazy Credential Delivery** — Credentials are pre-authorized at spawn (scoped to the task's required skills) but not delivered until the agent explicitly requests them at runtime. Minimizes exposure window.

**Stateless by Design** — This component owns no persistent storage. All state is delegated to the Memory Component. Enables clean crash recovery and horizontal scaling.

**Single Comms Gateway** — All inter-component messaging flows through `internal/comms`. No module bypasses it. Simplifies auditing, retry logic, and integration testing.

**MicroVM Isolation** — Every agent runs in its own Firecracker microVM. A compromised agent cannot reach another agent or the host.

---

## Getting Started

### Prerequisites

- Go 1.22+
- NATS Server (for integration testing)
- Access to OpenBao instance (for credential integration)
- Firecracker binary (for microVM lifecycle — stub available for development)

### Install & Run

```bash
git clone https://github.com/your-org/aegis-agents
cd aegis-agents
go mod tidy
go build ./...
```

### Configuration

All configuration is environment-based. Copy and edit the example:

```bash
cp config/.env.example .env
```

Key variables:

| Variable | Description |
|----------|-------------|
| `NATS_URL` | NATS JetStream endpoint (e.g., `nats://localhost:4222`) |
| `OPENBAO_ADDR` | OpenBao API address |
| `MEMORY_COMPONENT_ADDR` | Memory Component service address |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` |
| `FIRECRACKER_BIN` | Path to Firecracker binary (set to `stub` in dev) |

### Run Tests

```bash
# Unit tests (no external dependencies)
go test ./internal/...

# Integration tests (requires NATS)
go test ./test/integration/...
```

---

## Project Structure

```
aegis-agents/
├── CLAUDE.md                  # AI development briefing (read before coding)
├── README.md                  # This file
├── go.mod
├── go.sum
├── cmd/
│   └── aegis-agents/
│       └── main.go
├── internal/
│   ├── comms/                 # M1: Communications Interface
│   ├── factory/               # M2: Agent Factory
│   ├── registry/              # M3: Agent Registry
│   ├── skills/                # M4: Skill Hierarchy Manager
│   ├── credentials/           # M5: Credential Broker
│   ├── lifecycle/             # M6: Lifecycle Manager
│   └── memory/                # M7: Memory Interface
├── pkg/
│   └── types/                 # Shared types (TaskSpec, AgentRecord, etc.)
├── config/
│   └── config.go
├── test/
│   └── integration/
└── docs/
    ├── EDD.pdf
    └── ADR/
        ├── 001-native-go.md
        └── 002-centralized-comms.md
```

---

## External Integrations

| Component | Protocol | Our Role |
|-----------|----------|----------|
| Orchestrator | NATS JetStream | Receive `task_spec`, publish `task_result` and `capability_response` |
| Communications Component | NATS JetStream | All external messages route through this transport |
| Credential Vault (OpenBao) | HTTP API | Request permission scopes at spawn; validate credential requests at runtime |
| Memory Component | Internal API | Write tagged agent state; read filtered context slices |

---

## Documentation

Full design documentation lives in `/docs/`:

- **EDD** — Engineering Design Document covering all module specs, data flows, and interface contracts
- **ADR-001** — Native Go implementation rationale
- **ADR-002** — Centralized communications gateway decision

---

## Contributing

This component is part of Aegis OS. Before contributing, read `CLAUDE.md` for architectural constraints and `docs/EDD.pdf` for the full design spec. All PRs must maintain the module boundaries defined in the EDD.

---

## License

See [LICENSE](LICENSE).
