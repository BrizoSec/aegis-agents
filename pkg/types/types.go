package types

import "time"

// TaskSpec is received from the Orchestrator via the Comms Interface.
type TaskSpec struct {
	TaskID         string            `json:"task_id"`
	RequiredSkills []string          `json:"required_skills"` // domain names only
	Metadata       map[string]string `json:"metadata"`
	TraceID        string            `json:"trace_id"`
}

// AgentRecord is the catalog entry stored in the Registry.
type AgentRecord struct {
	AgentID       string    `json:"agent_id"`
	State         string    `json:"state"` // idle | active | terminated
	SkillDomains  []string  `json:"skill_domains"`
	PermissionSet []string  `json:"permission_set"`
	AssignedTask  string    `json:"assigned_task,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SkillSpec is the leaf-level parameter schema for a skill command.
type SkillSpec struct {
	Parameters map[string]ParameterDef `json:"parameters"`
}

// ParameterDef describes a single parameter in a skill's call spec.
type ParameterDef struct {
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

// SkillNode is a node in the three-level skill hierarchy (domain → command → spec).
type SkillNode struct {
	Name     string                `json:"name"`
	Level    string                `json:"level"` // domain | command | spec
	Children map[string]*SkillNode `json:"children,omitempty"`
	Spec     *SkillSpec            `json:"spec,omitempty"` // only at leaf level
}

// MemoryWrite is the tagged payload sent to the Memory Component.
type MemoryWrite struct {
	AgentID   string            `json:"agent_id"`
	SessionID string            `json:"session_id"`
	DataType  string            `json:"data_type"`
	TTLHint   int               `json:"ttl_hint_seconds"`
	Payload   interface{}       `json:"payload"`
	Tags      map[string]string `json:"tags"`
}

// Envelope is the standard NATS message wrapper for all inter-component messages.
type Envelope struct {
	MessageID   string      `json:"message_id"`
	Source      string      `json:"source"`
	Destination string      `json:"destination"`
	Timestamp   time.Time   `json:"timestamp"`
	Payload     interface{} `json:"payload"`
	TraceID     string      `json:"trace_id"`
}

// TaskResult is published to the Orchestrator on task completion.
type TaskResult struct {
	TaskID  string      `json:"task_id"`
	AgentID string      `json:"agent_id"`
	Success bool        `json:"success"`
	Output  interface{} `json:"output,omitempty"`
	Error   string      `json:"error,omitempty"`
	TraceID string      `json:"trace_id"`
}

// StatusUpdate is published to the Orchestrator for progress events.
type StatusUpdate struct {
	TaskID  string `json:"task_id"`
	AgentID string `json:"agent_id"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
	TraceID string `json:"trace_id"`
}

// CapabilityResponse answers an Orchestrator capability query.
type CapabilityResponse struct {
	QueryID  string   `json:"query_id"`
	Domains  []string `json:"domains"`
	HasMatch bool     `json:"has_match"`
	TraceID  string   `json:"trace_id"`
}
