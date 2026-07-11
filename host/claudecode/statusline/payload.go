package statusline

// Payload is Claude Code's documented statusLine stdin JSON contract. Only
// SessionID and Cwd are load-bearing for the two known consumers
// (ouroboros, pogopin) today; the remaining fields are modeled because
// Claude Code documents them and a future consumer may need them (e.g. Cost
// for a token-budget segment) — see
// https://code.claude.com/docs/en/statusline.
type Payload struct {
	HookEventName  string      `json:"hook_event_name"`
	SessionID      string      `json:"session_id"`
	TranscriptPath string      `json:"transcript_path"`
	Cwd            string      `json:"cwd"`
	Model          Model       `json:"model"`
	Workspace      Workspace   `json:"workspace"`
	Version        string      `json:"version"`
	OutputStyle    OutputStyle `json:"output_style"`
	Cost           Cost        `json:"cost"`
}

// Model identifies the resolved model the current session is running.
type Model struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// Workspace carries the current and project directories for the session.
type Workspace struct {
	CurrentDir string `json:"current_dir"`
	ProjectDir string `json:"project_dir"`
}

// OutputStyle names the session's active output style.
type OutputStyle struct {
	Name string `json:"name"`
}

// Cost carries session cost/usage counters Claude Code reports alongside
// the statusLine invocation.
type Cost struct {
	TotalCostUSD       float64 `json:"total_cost_usd"`
	TotalDurationMS    int64   `json:"total_duration_ms"`
	TotalAPIDurationMS int64   `json:"total_api_duration_ms"`
	TotalLinesAdded    int64   `json:"total_lines_added"`
	TotalLinesRemoved  int64   `json:"total_lines_removed"`
}
