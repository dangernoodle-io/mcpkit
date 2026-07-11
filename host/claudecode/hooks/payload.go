package hooks

import "encoding/json"

// Common fields are present on every Claude Code hook invocation's stdin
// JSON, per Claude Code's documented hook input schema.
type Common struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	HookEventName  string `json:"hook_event_name"`
}

// StopPayload is the Stop event's stdin payload.
type StopPayload struct {
	Common
	StopHookActive bool `json:"stop_hook_active"`
}

// SubagentStopPayload is the SubagentStop event's stdin payload.
type SubagentStopPayload struct {
	Common
	AgentID              string `json:"agent_id"`
	AgentType            string `json:"agent_type"`
	LastAssistantMessage string `json:"last_assistant_message"`
	StopHookActive       bool   `json:"stop_hook_active"`
}

// SubagentStartPayload is the SubagentStart event's stdin payload.
type SubagentStartPayload struct {
	Common
	AgentType string `json:"agent_type"`
	Prompt    string `json:"prompt"`
}

// UserPromptSubmitPayload is the UserPromptSubmit event's stdin payload.
type UserPromptSubmitPayload struct {
	Common
	Prompt string `json:"prompt"`
}

// PreToolUsePayload is the PreToolUse event's stdin payload. ToolInput is
// left as json.RawMessage: its shape varies per tool, and handlers that
// need it typically re-decode into a tool-specific struct.
type PreToolUsePayload struct {
	Common
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// PostToolUsePayload is the PostToolUse event's stdin payload. ToolInput
// and ToolResponse are left as json.RawMessage for the same reason as
// PreToolUsePayload.ToolInput.
type PostToolUsePayload struct {
	Common
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

// PreCompactPayload is the PreCompact event's stdin payload.
type PreCompactPayload struct {
	Common
	Trigger            string `json:"trigger"`
	CustomInstructions string `json:"custom_instructions"`
}

// SessionStartPayload is the SessionStart event's stdin payload.
type SessionStartPayload struct {
	Common
	Source string `json:"source"`
}
