package types

import "time"

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
type ConfigLoadedMsg struct {
	ModelName string
	URL       string
	Err       error
}

type StreamMsg struct {
	ID    string
	Token string
	Done  bool
}

type StreamErrorMsg struct {
	ID    string
	Error string
}

type StreamTokenMsg struct {
	ID    string
	Token string
	Done  bool
}

type ViewportContentMsg struct {
	Content string
}

type ModelsLoadedMsg struct {
	Models []string
	Err    error
}

type ScrollToBottomMsg struct{}
