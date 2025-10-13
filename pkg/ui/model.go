package ui

import (
	"fmt"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thebug/lab/eko/v3/pkg/config"
	"github.com/thebug/lab/eko/v3/pkg/ollama"
	"github.com/thebug/lab/eko/v3/pkg/types"
)

var (
	accentColor   = lipgloss.AdaptiveColor{Light: "#fe3f01", Dark: "#fe3f01"}
	defaultColor  = lipgloss.AdaptiveColor{Light: "#BCBCBC", Dark: "#BCBCBC"}
	subtleColor   = lipgloss.AdaptiveColor{Light: "#555555", Dark: "#555555"}
	amoblackColor = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#000000"}
)

// Model represents the main application model
type Model struct {
	state           types.State
	viewMode        types.ViewMode
	messages        []types.Message
	viewport        viewport.Model
	input           textinput.Model
	spinner         spinner.Model
	modelName       string
	configManager   *config.Manager
	ollamaClient    *ollama.Client
	width           int
	height          int
	modelList       []string
	selectedIdx     int
	saveName        string
	streaming       bool
	isThinking      bool
	currentStreamID string

	// For gg / G navigation
	lastKey  string
	keyTimer time.Time

	// Real-time streaming
	msgChan chan tea.Msg

	// For yank mode
	yankInput       string
	yankStatus      string    // For showing success/failure messages
	yankStatusTimer time.Time // For auto-clearing status messages
}

// NewModel creates a new application model
func NewModel() Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(accentColor)
	ti.Placeholder = "Type your message..."
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(subtleColor)

	vp := viewport.New(80, 20)
	vp.SetContent("")

	// Ensure viewport has minimum dimensions
	if vp.Width < 20 {
		vp.Width = 20
	}
	if vp.Height < 10 {
		vp.Height = 10
	}

	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(accentColor)

	return Model{
		state:           types.NormalState,
		viewMode:        types.VerboseMode,
		viewport:        vp,
		input:           ti,
		spinner:         s,
		modelName:       config.DefaultModel,
		configManager:   config.NewManager(),
		ollamaClient:    ollama.NewClient(),
		streaming:       false,
		isThinking:      false,
		currentStreamID: "",
		lastKey:         "",
		msgChan:         make(chan tea.Msg, 100), // Buffered channel for streaming messages
		yankInput:       "",
		yankStatus:      "",
		yankStatusTimer: time.Time{},
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.configManager.LoadConfig(),
		m.initializeViewport(),
		m.updateViewportContent(),
	)
}

// Update handles model updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	// Handle messages from the streaming channel
	select {
	case streamMsg := <-m.msgChan:
		// Process streaming message
		switch streamMsg := streamMsg.(type) {
		case types.TokenMsg:
			if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" && m.messages[len(m.messages)-1].ID == streamMsg.ID {
				m.messages[len(m.messages)-1].Content += streamMsg.Token
				// Direct update instead of throttled redraw to prevent crashes
				cmds = append(cmds, m.updateViewportContent())
			}
		case types.GenerationStartMsg:
			m.isThinking = true
			m.currentStreamID = streamMsg.ID
			cmds = append(cmds, m.spinner.Tick)
		case types.GenerationDoneMsg:
			m.isThinking = false
			m.streaming = false
			m.currentStreamID = ""
			cmds = append(cmds, m.updateViewportContent())
		case types.StreamErrorMsg:
			if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
				m.messages[len(m.messages)-1].Content = fmt.Sprintf("Error: %s", streamMsg.Error)
				m.viewport.GotoBottom()
			}
			m.streaming = false
			m.isThinking = false
		case types.CancelStreamMsg:
			// Handle stream cancellation
			if m.currentStreamID == streamMsg.ID {
				m.isThinking = false
				m.streaming = false
				m.currentStreamID = ""
				if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
					m.messages[len(m.messages)-1].Content += " [Stream cancelled]"
				}
				cmds = append(cmds, m.updateViewportContent())
			}
		}
	default:
		// No message from channel, continue with normal processing
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle state transitions first
		justTransitioned := false
		if m.state == types.NormalState {
			switch msg.String() {
			case "i":
				// Enter insert mode with empty input (current behavior)
				m.state = types.InsertState
				m.input.Focus()
				m.input.Prompt = ""
				m.input.SetValue("")
				justTransitioned = true
				// Don't process the 'i' key by input
				break
			case ":":
				m.state = types.CommandState
				m.input.Focus()
				m.input.Prompt = ":"
				m.input.SetValue("")
				justTransitioned = true
				// Don't process the ':' key by input
				break
			case "y":
				m.state = types.YankCodeState
				justTransitioned = true
				// Don't process the 'y' key further
				break
			case "o":
				// Enter insert mode with last user message prefilled
				m.state = types.InsertState
				m.input.Focus()
				m.input.Prompt = ""

				// Find the last user message and prefilled it
				lastUserMessage := m.getLastUserMessage()
				m.input.SetValue(lastUserMessage)
				justTransitioned = true
				// Don't process the 'o' key by input
				break
			case "O":
				// Enter insert mode with last assistant message prefilled
				m.state = types.InsertState
				m.input.Focus()
				m.input.Prompt = ""

				// Find the last assistant message and prefilled it
				lastAssistantMessage := m.getLastAssistantMessage()
				m.input.SetValue(lastAssistantMessage)
				justTransitioned = true
				// Don't process the 'O' key by input
				break
			case "tab":
				// Toggle focus
				break
			case "ctrl+c":
				// Cancel current stream if active, otherwise quit
				if m.isThinking && m.currentStreamID != "" {
					cmds = append(cmds, m.cancelStream(m.currentStreamID))
				} else {
					cmds = append(cmds, tea.Quit)
				}
				break
			case "q":
				cmds = append(cmds, tea.Quit)
				break
			// Navigation: G and gg
			case "G":
				if len(m.messages) > 0 {
					m.viewport.GotoBottom()
				}
				// reset lastKey
				m.lastKey = ""
				break
			case "g":
				// double-tap 'g' quickly => top
				now := time.Now()
				if m.lastKey == "g" && now.Sub(m.keyTimer) <= 300*time.Millisecond {
					m.viewport.GotoTop()
					m.lastKey = ""
				} else {
					m.lastKey = "g"
					m.keyTimer = now
				}
				break
			}
		} else {
			// Handle other states (insert, command, yank, config)
			switch m.state {
			case types.InsertState:
				// Handle insert state specific keys
				// Check for Shift+Enter first - try multiple possible representations
				keyStr := msg.String()

				// Temporary debug: Show what key is being pressed
				if keyStr != "enter" && msg.Type == tea.KeyEnter {
					currentValue := m.input.Value()
					m.input.SetValue(currentValue + "[DEBUG:" + keyStr + "]")
					justTransitioned = true
				} else if keyStr == "shift+enter" || keyStr == "shift+return" ||
					(msg.Type == tea.KeyEnter && len(keyStr) > 5) {
					// Shift+Enter: Add new line to input
					currentValue := m.input.Value()
					m.input.SetValue(currentValue + "\n")
					// Don't process this key further
					justTransitioned = true
				} else if keyStr == "enter" {
					// Regular Enter: Send message
					if m.input.Value() == "" {
						// Do nothing if input is empty
					} else {
						// Cancel any existing stream before starting new one
						if m.isThinking && m.currentStreamID != "" {
							cmds = append(cmds, m.cancelStream(m.currentStreamID))
						}

						// Add user message
						id := generateID(len(m.messages))
						userMsg := types.Message{ID: id, Role: "user", Content: m.input.Value(), IsCollapsed: false, Timestamp: time.Now()}
						m.messages = append(m.messages, userMsg)

						// Add placeholder AI message
						aiId := generateID(len(m.messages))
						aiMsg := types.Message{ID: aiId, Role: "assistant", Content: "", IsCollapsed: false, Timestamp: time.Now()}
						m.messages = append(m.messages, aiMsg)

						// Start real-time streaming response
						m.streaming = true
						m.isThinking = true
						m.currentStreamID = aiId
						cmds = append(cmds, m.startRealtimeStream(aiId), m.updateViewportContent(), m.scrollToBottom())
						m.state = types.NormalState
						m.input.Reset()
					}
				} else if msg.String() == "esc" {
					m.state = types.NormalState
					m.input.Reset()
				}
			case types.CommandState:
				// Handle command state specific keys
				if msg.String() == "enter" {
					command := m.input.Value()
					m.input.Reset()
					cmd := m.handleCommand(command)
					if cmd != nil {
						cmds = append(cmds, cmd)
					}
					// Don't reset to normal state here - let handleCommand decide the state
				} else if msg.String() == "esc" {
					m.state = types.NormalState
					m.input.Reset()
				}
			case types.YankState:
				// Handle yank state
				if len(msg.String()) == 2 {
					// Try to yank message with this ID
					for _, message := range m.messages {
						if message.ID == msg.String() {
							clipboard.WriteAll(message.Content)
							break
						}
					}
					m.state = types.NormalState
				} else if msg.String() == "esc" {
					m.state = types.NormalState
				}
			case types.YankCodeState:
				// Handle yank code state - collect full code block ID
				// Capture ALL keys exclusively in yank mode
				keyStr := msg.String()
				if keyStr == "enter" {
					// Process the yank input
					if m.yankInput != "" {
						// Try to find and copy the code block
						if block, exists := GetCodeBlock(m.yankInput); exists {
							err := clipboard.WriteAll(block.Content)
							if err != nil {
								m.yankStatus = "✖ Failed to copy"
							} else {
								m.yankStatus = "✔ Copied " + m.yankInput
							}
							m.yankStatusTimer = time.Now()
						} else {
							m.yankStatus = "✖ Invalid code ID"
							m.yankStatusTimer = time.Now()
						}
					}
					m.yankInput = ""
					m.state = types.NormalState
				} else if keyStr == "esc" {
					m.yankInput = ""
					m.state = types.NormalState
				} else if len(keyStr) == 1 {
					// Append ANY single character to yank input (capture all keys)
					m.yankInput += keyStr
				} else if keyStr == "backspace" || keyStr == "delete" {
					// Remove last character from yank input
					if len(m.yankInput) > 0 {
						m.yankInput = m.yankInput[:len(m.yankInput)-1]
					}
				}
				// CRITICAL: Don't process any other keys in yank mode
				justTransitioned = true
				break
			case types.ConfigState:
				// Handle config state
				switch msg.String() {
				case "j":
					if m.selectedIdx < len(m.modelList)-1 {
						m.selectedIdx++
					}
				case "k":
					if m.selectedIdx > 0 {
						m.selectedIdx--
					}
				case "enter":
					if m.selectedIdx < len(m.modelList) {
						m.modelName = m.modelList[m.selectedIdx]
						m.state = types.NormalState
						cmds = append(cmds, m.configManager.SaveConfig(m.modelName))
					}
				case "esc":
					m.state = types.NormalState
				}
			}
		}

		// Process input if we're in insert or command state, but skip if we just transitioned
		if !justTransitioned {
			if m.state == types.InsertState {
				// Skip processing Shift+Enter for new lines
				if msg.String() != "shift+enter" && msg.String() != "shift+return" &&
					!(msg.Type == tea.KeyEnter && len(msg.String()) > 5) {
					m.input, cmd = m.input.Update(msg)
					cmds = append(cmds, cmd)
				}
			} else if m.state == types.CommandState {
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		// Account for header (1 line) and input (2 lines height)
		m.viewport.Height = msg.Height - 3
		cmds = append(cmds, m.updateViewportContent())
		return m, tea.Batch(cmds...)

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case types.ConfigLoadedMsg:
		if msg.Err == nil {
			if msg.ModelName != "" {
				m.modelName = msg.ModelName
			}
			if msg.URL != "" {
				m.ollamaClient.BaseURL = msg.URL
			}
		}
		// Fetch models after config is loaded and URL is set
		cmds = append(cmds, m.ollamaClient.FetchModels())

	case types.ModelsLoadedMsg:
		if msg.Err == nil && len(msg.Models) > 0 {
			m.modelList = msg.Models
		} else {
			// Fallback to default models if Ollama is not available
			m.modelList = []string{"dolphin-phi", "llama2-uncensored", "mistral", "qwen3:1.7b", "gemma3"}
		}

	case types.StreamMsg:
		// Append token to the last assistant message
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
			m.messages[len(m.messages)-1].Content += msg.Token
		}

		// Continue streaming if not done
		if !msg.Done {
			cmds = append(cmds, m.continueStream(msg.ID))
		} else {
			m.streaming = false
		}

		// Update viewport content (no auto-scroll for assistant responses)
		cmds = append(cmds, m.updateViewportContent())

	case types.StreamTokenMsg:
		// Handle real-time streaming tokens
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
			m.messages[len(m.messages)-1].Content += msg.Token
		}

		// Continue streaming if not done
		if !msg.Done {
			cmds = append(cmds, m.continueStreamRealtime(msg.ID))
		} else {
			m.streaming = false
		}

		// Update viewport content (no auto-scroll for assistant responses)
		cmds = append(cmds, m.updateViewportContent())

	// New real-time streaming message handlers
	case types.TokenMsg:
		// Handle individual token updates
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" && m.messages[len(m.messages)-1].ID == msg.ID {
			m.messages[len(m.messages)-1].Content += msg.Token
			// Direct update instead of throttled redraw to prevent crashes
			cmds = append(cmds, m.updateViewportContent())
		}

	case types.GenerationStartMsg:
		// Mark that generation has started
		m.isThinking = true
		m.currentStreamID = msg.ID
		cmds = append(cmds, m.spinner.Tick)

	case types.GenerationDoneMsg:
		// Mark that generation is complete
		m.isThinking = false
		m.streaming = false
		m.currentStreamID = ""
		// Final redraw and scroll to bottom
		cmds = append(cmds, m.updateViewportContent(), m.scrollToBottom())

	case types.RedrawMsg:
		// Handle redraw message
		cmds = append(cmds, m.updateViewportContent())

	case types.StreamErrorMsg:
		// Handle streaming error
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
			m.messages[len(m.messages)-1].Content = fmt.Sprintf("Error: %s", msg.Error)
			m.viewport.GotoBottom()
		}
		m.streaming = false

	case types.ViewportContentMsg:
		// Update viewport content
		m.viewport.SetContent(msg.Content)
		// Only scroll to bottom for user prompts, not assistant responses
		// (This will be handled by the specific message type that triggers this)

	case types.ScrollToBottomMsg:
		// Force scroll to bottom
		m.viewport.GotoBottom()

	}

	// Reset yank status if it's been shown for more than 3 seconds
	if m.yankStatus != "" && time.Since(m.yankStatusTimer) >= 3*time.Second {
		m.yankStatus = ""
	}

	// Update viewport for scrolling only when not in insert mode
	if m.state != types.InsertState {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// getLastUserMessage returns the content of the last user message, or empty string if none exists
// This is used by the 'o' key to prefilled the input with the previous user prompt
func (m Model) getLastUserMessage() string {
	// Search messages from the end backwards to find the last user message
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "user" {
			return m.messages[i].Content
		}
	}
	return "" // No user message found - will result in empty input
}

// getLastAssistantMessage returns the content of the last assistant message, or empty string if none exists
// This is used by the 'O' key to prefilled the input with the previous assistant response
func (m Model) getLastAssistantMessage() string {
	// Search messages from the end backwards to find the last assistant message
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "assistant" {
			return m.messages[i].Content
		}
	}
	return "" // No assistant message found - will result in empty input
}

// View renders the model
func (m Model) View() string {
	switch m.state {
	case types.ConfigState:
		return m.renderModelList()
	default:
		return m.renderMainView()
	}
}

// handleInsertState handles input in insert state
func (m *Model) handleInsertState(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		if m.input.Value() == "" {
			return nil
		}

		// Add user message
		id := generateID(len(m.messages))
		userMsg := types.Message{ID: id, Role: "user", Content: m.input.Value(), IsCollapsed: false, Timestamp: time.Now()}
		m.messages = append(m.messages, userMsg)

		// Add placeholder AI message
		aiId := generateID(len(m.messages))
		aiMsg := types.Message{ID: aiId, Role: "assistant", Content: "", IsCollapsed: false, Timestamp: time.Now()}
		m.messages = append(m.messages, aiMsg)

		// Start streaming real-time response
		m.streaming = true
		cmds := []tea.Cmd{m.streamResponseRealtime(aiMsg.ID), m.updateViewportContent(), m.scrollToBottom()}
		m.state = types.NormalState
		m.input.Reset()
		return tea.Batch(cmds...)

	case "esc":
		m.state = types.NormalState
		m.input.Reset()
		return nil
	}

	// For all other keys, let the input handle them
	// This will be handled in the main Update function
	return nil
}

// handleCommandState handles input in command state
func (m *Model) handleCommandState(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		command := m.input.Value()
		m.input.Reset()
		cmd := m.handleCommand(command)
		return cmd

	case "esc":
		m.state = types.NormalState
		m.input.Reset()
		return nil
	}
	return nil
}

// handleYankState handles input in yank state
func (m *Model) handleYankState(msg tea.KeyMsg) tea.Cmd {
	if len(msg.String()) == 2 {
		// Try to yank message with this ID
		for _, message := range m.messages {
			if message.ID == msg.String() {
				clipboard.WriteAll(message.Content)
				break
			}
		}
		m.state = types.NormalState
		return nil
	} else if msg.String() == "esc" {
		m.state = types.NormalState
		return nil
	}
	return nil
}

// handleConfigState handles input in config state
func (m *Model) handleConfigState(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "j":
		if m.selectedIdx < len(m.modelList)-1 {
			m.selectedIdx++
		}
		return nil

	case "k":
		if m.selectedIdx > 0 {
			m.selectedIdx--
		}
		return nil

	case "enter":
		if m.selectedIdx < len(m.modelList) {
			m.modelName = m.modelList[m.selectedIdx]
			m.state = types.NormalState
			return m.configManager.SaveConfig(m.modelName)
		}
		return nil

	case "esc":
		m.state = types.NormalState
		return nil
	}
	return nil
}
