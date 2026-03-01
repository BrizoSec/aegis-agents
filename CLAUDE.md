# CLAUDE.md — Aegis Agents Component

This file is the persistent project briefing for Claude Code. Read this at the start of every session before writing any code.

---

## What This Repo Is

`aegis-agents` is the **Agents Component** of Aegis OS — a distributed, hardened operating system purpose-built for running autonomous AI agents. This repo is one of several components built by separate teams. We own the agent lifecycle end-to-end: receiving tasks, provisioning agents, managing skills and credentials, and maintaining the agent registry.

We do **not** own: task routing/matching (Orchestrator), persistent storage (Memory Component), secret storage (Credential Vault / OpenBao), or message transport (Communications Component / NATS JetStream). We integrate with all four.

---

## Language & Stack

- **Language:** Go (native — no Python, no Node)
- **Transport:** NATS JetStream (via Communications Component — never call NATS directly from business logic)
- **Secrets:** OpenBao API (internal network only)
- **Isolation:** Firecracker microVMs — one VM per agent
- **Storage:** None owned. All persistence delegated to the Memory Component via the Memory Interface module.

---

## Core Architecture Principles

These are settled decisions. Do not re-litigate them.

**1. Single Communications Gateway**
All inter-component messaging (to/from Orchestrator, Memory, Vault, Comms) flows through the `comms` module. No internal module calls external components directly. This is the primary integration boundary.

**2. MicroVM per Agent**
Every agent runs in its own Firecracker microVM. A compromised agent cannot reach another agent or the host OS. The Lifecycle Manager owns VM spawn and teardown.

**3. Progressive Skill Disclosure**
Agents do not receive their full skill set at spawn. Skills are organized in a three-level hierarchy (domain → command → parameter spec). Agents discover skills on demand as they need them. Pre-loading all capabilities degrades performance (context rot). The Skill Hierarchy Manager enforces this.

**4. Lazy Credential Delivery**
Credentials are pre-authorized at spawn time (permission set scoped to the task) but not delivered until the agent explicitly requests a specific credential during skill invocation. This minimizes the exposure window and enforces least privilege. The Credential Broker owns this two-phase model.

**5. Stateless Component**
The Agents Component owns no persistent storage. All state that must survive restarts (agent registry, skill definitions, session history) is written to the Memory Component via the Memory Interface. Writes are surgical — only explicitly tagged, resolved data is persisted. Never dump full session state.

**6. Orchestrator Owns Task Matching**
We do not decide which task goes to which agent. The Orchestrator does. We respond to capability queries (does an agent with these skills exist?) and provision agents when asked. Do not build task routing logic into this component.

---

## Module Map

| Module | Package | Responsibility |
|---|---|---|
| M1 — Communications Interface | `internal/comms` | Single inbound/outbound NATS gateway. All external messages enter and leave here. |
| M2 — Agent Factory | `internal/factory` | Central coordinator. Receives task specs, queries registry, initiates provisioning for new agents. |
| M3 — Agent Registry | `internal/registry` | In-memory catalog of agents (ID, state, skill domains, credential permission set). Backed by Memory Component. |
| M4 — Skill Hierarchy Manager | `internal/skills` | Owns the skills tree. Serves skill discovery on demand. Never pre-loads leaf-level detail. |
| M5 — Credential Broker | `internal/credentials` | Two-phase credential model: pre-authorize at spawn, lazy delivery at runtime. Talks to OpenBao. |
| M6 — Lifecycle Manager | `internal/lifecycle` | Spawns and terminates Firecracker microVMs. Health monitoring, crash recovery, state updates. |
| M7 — Memory Interface | `internal/memory` | Thin layer for all Memory Component interactions. Enforces tagged writes, filtered reads. |

---

## External Interface Contracts

### Orchestrator (via Comms / NATS)
- **Inbound:** `task_spec` — task assignment with required skill domains and metadata
- **Outbound:** `task_result` — completion payload; `status_update` — progress events; `capability_response` — answer to capability queries

### Credential Vault (OpenBao API)
- **Phase 1 (spawn):** POST permission scope request → receive scoped vault token
- **Phase 2 (runtime):** Agent requests specific secret → Credential Broker validates against pre-approved set → returns secret value
- Security levels: L0 (public) through L4 (vault-reserved). Skill domain dictates required level.

### Memory Component (via Memory Interface)
- **Writes:** Tagged payloads only. Tag includes: agent ID, session ID, data type, TTL hint.
- **Reads:** Filtered slices by agent ID + context tag. Never request full agent history.

### Communications Component (NATS JetStream)
- All messages use the standard envelope: `{ message_id, source, destination, timestamp, payload, trace_id }`
- At-least-once delivery for task results and status. At-most-once for heartbeats.
- Do not publish to NATS directly. Always route through `internal/comms`.

---

## Skill Hierarchy Schema

```
domain (e.g., "web", "data", "comms", "storage")
  └── command (e.g., "web.fetch", "web.parse")
        └── parameter_spec (full schema: types, required fields, validation rules)
```

Agents receive only the domain name at spawn. They query for available commands within a domain when they need to act. They query for parameter specs only when constructing a specific call. The `skills` package enforces this three-step drill-down.

---

## Agent Lifecycle

```
task_spec received
  → Factory queries Registry for capable agent
  → [Match found] → assign task to existing agent
  → [No match] → Factory initiates provisioning:
      1. Skill Hierarchy Manager: resolve entry-point skill domain
      2. Credential Broker: pre-authorize permission set for task
      3. Lifecycle Manager: spawn Firecracker microVM
      4. Inject: minimal context + skill domain entry point + credential pointer
      5. Registry: register agent with state=active
  → Agent executes task (skill discovery and credential delivery on demand)
  → Agent completes → Factory collects result
  → Memory Interface: persist tagged outputs
  → Comms Interface: publish task_result to Orchestrator
  → Lifecycle Manager: terminate microVM
  → Registry: update agent state to idle or terminated
```

---

## Directory Structure

```
aegis-agents/
├── CLAUDE.md                  # This file
├── README.md
├── go.mod
├── go.sum
├── cmd/
│   └── aegis-agents/
│       └── main.go            # Entry point
├── internal/
│   ├── comms/                 # M1: Communications Interface
│   ├── factory/               # M2: Agent Factory
│   ├── registry/              # M3: Agent Registry
│   ├── skills/                # M4: Skill Hierarchy Manager
│   ├── credentials/           # M5: Credential Broker
│   ├── lifecycle/             # M6: Lifecycle Manager
│   └── memory/                # M7: Memory Interface
├── pkg/
│   └── types/                 # Shared types: TaskSpec, AgentRecord, SkillNode, etc.
├── config/
│   └── config.go              # Environment-based config (NATS URL, OpenBao addr, etc.)
└── docs/
    ├── EDD.pdf                # Engineering Design Document
    └── ADR/
        ├── 001-native-go.md
        └── 002-centralized-comms.md
```

---

## Key Types (pkg/types)

When creating new types, ensure they conform to these core shapes:

```go
// TaskSpec — received from Orchestrator
type TaskSpec struct {
    TaskID       string            `json:"task_id"`
    RequiredSkills []string        `json:"required_skills"` // domain names only
    Metadata     map[string]string `json:"metadata"`
    TraceID      string            `json:"trace_id"`
}

// AgentRecord — stored in Registry
type AgentRecord struct {
    AgentID       string    `json:"agent_id"`
    State         string    `json:"state"` // idle | active | terminated
    SkillDomains  []string  `json:"skill_domains"`
    PermissionSet []string  `json:"permission_set"`
    AssignedTask  string    `json:"assigned_task,omitempty"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}

// SkillNode — node in the skill hierarchy
type SkillNode struct {
    Name     string               `json:"name"`
    Level    string               `json:"level"` // domain | command | spec
    Children map[string]*SkillNode `json:"children,omitempty"`
    Spec     *SkillSpec           `json:"spec,omitempty"` // only at leaf level
}

// MemoryWrite — payload sent to Memory Component
type MemoryWrite struct {
    AgentID   string            `json:"agent_id"`
    SessionID string            `json:"session_id"`
    DataType  string            `json:"data_type"`
    TTLHint   int               `json:"ttl_hint_seconds"`
    Payload   interface{}       `json:"payload"`
    Tags      map[string]string `json:"tags"`
}
```

---

## Development Guidelines

- **Interfaces first.** Define the interface for each module before implementing it. This allows parallel development and clean mocking in tests.
- **No direct external calls from business logic.** Factory, Registry, Skills, Credentials, and Lifecycle must only communicate externally through `internal/comms` or `internal/memory`.
- **Error propagation.** Use structured errors with context. Every error should carry the module name and operation that produced it.
- **Config via environment.** No hardcoded addresses. All external endpoints (NATS, OpenBao, Memory Component) loaded from environment via `config/config.go`.
- **Testing.** Each module must have unit tests with mocked dependencies. Integration tests live in `/test/integration/` and require a running NATS instance.
- **Logging.** Structured JSON logs. Every log entry must include `trace_id` when processing a task.

---

## What We Are Building First

Implement in this order:
1. `pkg/types` — shared type definitions
2. `config/config.go` — environment config
3. `internal/comms` — Communications Interface (stub NATS until integration)
4. `internal/registry` — Agent Registry (in-memory first, Memory Component integration second)
5. `internal/skills` — Skill Hierarchy Manager
6. `internal/credentials` — Credential Broker
7. `internal/lifecycle` — Lifecycle Manager (stub microVM until Firecracker integration)
8. `internal/memory` — Memory Interface
9. `internal/factory` — Agent Factory (wires all modules together)
10. `cmd/aegis-agents/main.go` — Entry point and wiring
