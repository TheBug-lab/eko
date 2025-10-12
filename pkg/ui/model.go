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
	state        types.State
	viewMode     types.ViewMode
	messages     []types.Message
	viewport     viewport.Model
	input        textinput.Model
	spinner      spinner.Model
	modelName    string
	configManager *config.Manager
	ollamaClient *ollama.Client
	width        int
	height       int
	modelList    []string
	selectedIdx  int
	saveName     string
	streaming    bool

	// For gg / G navigation
	lastKey  string
	keyTimer time.Time
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
		state:         types.NormalState,
		viewMode:      types.VerboseMode,
		viewport:      vp,
		input:         ti,
		spinner:       s,
		modelName:     config.DefaultModel,
		configManager: config.NewManager(),
		ollamaClient:  ollama.NewClient(),
		streaming:     false,
		lastKey:       "",
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

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle state transitions first
		justTransitioned := false
		if m.state == types.NormalState {
			switch msg.String() {
			case "i":
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
				m.state = types.YankState
				m.input.Blur()
				break
			case "o":
				if m.viewMode == types.TLDRMode {
					// Expand current message
					if len(m.messages) > 0 {
						// Simple expansion of last message for demo
						if len(m.messages) > 0 && m.messages[len(m.messages)-1].IsCollapsed {
							m.messages[len(m.messages)-1].IsCollapsed = false
						}
					}
				}
				break
			case "tab":
				// Toggle focus
				break
			case "ctrl+c", "q":
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
				if msg.String() == "enter" {
					if m.input.Value() == "" {
						// Do nothing if input is empty
					} else {
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
						cmds = append(cmds, m.streamResponseRealtime(aiMsg.ID), m.updateViewportContent(), m.scrollToBottom())
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
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			} else if m.state == types.CommandState {
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		// Account for header (1 line) and input (3 lines with padding/borders)
		m.viewport.Height = msg.Height - 4
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

	// Reset lastKey if too slow
	if m.lastKey != "" && time.Since(m.keyTimer) > 300*time.Millisecond {
		m.lastKey = ""
	}

	// Update viewport for scrolling only when not in insert mode
	if m.state != types.InsertState {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
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
