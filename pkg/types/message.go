package types

import (
	"time"

	"github.com/thebug/lab/eko/v3/pkg/comfyui"
)

// Message represents a chat message
type Message struct {
	ID          string    `json:"id"`
	Role        string    `json:"role"`
	Content     string    `json:"content"`
	IsCollapsed bool      `json:"is_collapsed"`
	Timestamp   time.Time `json:"timestamp"`
}

// State represents the current application state
type State int

const (
	NormalState State = iota
	InsertState
	CommandState
	YankState
	YankCodeState  // New state for yanking code blocks
	ConfigState
	SaveState
)

// ViewMode represents the view mode for messages
type ViewMode int

const (
	VerboseMode ViewMode = iota
	TLDRMode
)

// Message types for tea.Cmd
type QueueStatusMsg struct {
	Count int
	Err   error
}

type ConfigLoadedMsg struct {
	ModelName    string
	URL          string
	ComfyUIURL   string
	WorkflowPath string
	Err          error
}

// Legacy streaming messages (kept for compatibility)
type StreamMsg struct {
	ID    string
	Token string
	Done  bool
}

type StreamErrorMsg struct {
	ID    string
	Error string
}

// Real-time streaming messages
type StreamTokenMsg struct {
	ID    string
	Token string
	Done  bool
}

// New streaming message types for real-time updates
type TokenMsg struct {
	ID    string
	Token string
}

type GenerationDoneMsg struct {
	ID string
}

type GenerationStartMsg struct {
	ID string
}

type RedrawMsg struct{}

type CancelStreamMsg struct {
	ID string
}

type ViewportContentMsg struct {
	Content string
}

type ModelsLoadedMsg struct {
	Models []string
	Err    error
}

type ScrollToBottomMsg struct{}

type ProgressMsg struct {
	ID     string
	Update comfyui.ProgressUpdate
}

// CodeBlock represents a code block with unique ID and metadata
type CodeBlock struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Content  string `json:"content"`
	MessageID string `json:"message_id"`
}

// YankModeMsg represents yank mode operations
type YankModeMsg struct {
	Action string // "enter", "exit", "copy"
	CodeID string // code block ID to copy
	Success bool  // whether the operation was successful
}
