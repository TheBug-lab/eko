package ui

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/thebug/lab/eko/v3/pkg/types"
)

// initializeViewport initializes the viewport
func (m Model) initializeViewport() tea.Cmd {
	return func() tea.Msg {
		// This will be called to initialize the viewport
		return nil
	}
}

// updateViewportContent updates the viewport content
func (m Model) updateViewportContent() tea.Cmd {
	// Capture current state
	messages := m.messages
	width := m.width
	height := m.height
	viewMode := m.viewMode
	streaming := m.streaming
	spinner := m.spinner
	
	return func() tea.Msg {
		// Add safety check to prevent panics, but use defaults if needed
		if width == 0 {
			width = 80
		}
		if height == 0 {
			height = 20
		}
		
		// Create a temporary model with captured state for rendering
		tempModel := m
		tempModel.messages = messages
		tempModel.width = width
		tempModel.height = height
		tempModel.viewMode = viewMode
		tempModel.streaming = streaming
		tempModel.spinner = spinner
		
		content := tempModel.renderMessages()
		return types.ViewportContentMsg{Content: content}
	}
}

// scrollToBottom scrolls the viewport to the bottom
func (m Model) scrollToBottom() tea.Cmd {
	return tea.Tick(time.Millisecond*10, func(time.Time) tea.Msg {
		return types.ScrollToBottomMsg{}
	})
}

// streamResponse streams a response from Ollama
func (m Model) streamResponse(id string) tea.Cmd {
	return func() tea.Msg {
		// Prepare messages for Ollama (exclude the empty assistant message we just added)
		messages := make([]types.Message, 0, len(m.messages)-1)
		for _, msg := range m.messages {
			if msg.ID != id { // Skip the empty assistant message
				messages = append(messages, msg)
			}
		}

		// Stream response from Ollama
		var fullResponse strings.Builder
		err := m.ollamaClient.StreamChat(m.modelName, messages, func(token string, done bool) {
			fullResponse.WriteString(token)
		})

		if err != nil {
			return types.StreamErrorMsg{ID: id, Error: err.Error()}
		}

		return types.StreamMsg{ID: id, Token: fullResponse.String(), Done: true}
	}
}

// streamResponseRealtime streams a response from Ollama in real-time
func (m Model) streamResponseRealtime(id string) tea.Cmd {
	return func() tea.Msg {
		// Prepare messages for Ollama (exclude the empty assistant message we just added)
		messages := make([]types.Message, 0, len(m.messages)-1)
		for _, msg := range m.messages {
			if msg.ID != id { // Skip the empty assistant message
				messages = append(messages, msg)
			}
		}

		// Stream response from Ollama with real-time updates
		var fullResponse strings.Builder
		err := m.ollamaClient.StreamChat(m.modelName, messages, func(token string, done bool) {
			fullResponse.WriteString(token)
		})

		if err != nil {
			return types.StreamErrorMsg{ID: id, Error: err.Error()}
		}

		return types.StreamTokenMsg{ID: id, Token: fullResponse.String(), Done: true}
	}
}

// continueStream continues streaming
func (m Model) continueStream(_ string) tea.Cmd {
	// This is just a placeholder since we're handling streaming in streamResponse
	return nil
}

// continueStreamRealtime continues real-time streaming
func (m Model) continueStreamRealtime(id string) tea.Cmd {
	return func() tea.Msg {
		// This will be called to continue streaming
		// We need to get the next token from the channel
		// For now, we'll return a simple continuation
		return types.StreamTokenMsg{ID: id, Token: "", Done: true}
	}
}

// handleCommand handles command input
func (m *Model) handleCommand(command string) tea.Cmd {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "config":
		m.state = types.ConfigState
		m.selectedIdx = 0
		// Find current model in list
		for i, model := range m.modelList {
			if model == m.modelName {
				m.selectedIdx = i
				break
			}
		}
		return nil

	case "save":
		if len(args) < 1 {
			m.state = types.NormalState
			return nil
		}

		filename := args[0]
		if !strings.HasSuffix(filename, ".json") {
			filename += ".json"
		}

		m.state = types.NormalState
		return func() tea.Msg {
			data, err := json.MarshalIndent(m.messages, "", "  ")
			if err != nil {
				// In a real app, we'd handle this error properly
				return nil
			}

			if err := os.WriteFile(filename, data, 0644); err != nil {
				// In a real app, we'd handle this error properly
				return nil
			}

			return nil
		}

	case "tldr":
		m.viewMode = types.TLDRMode
		// Collapse all messages except the last few
		for i := range m.messages {
			if len(m.messages[i].Content) > 100 {
				m.messages[i].IsCollapsed = true
			}
		}
		m.state = types.NormalState
		return nil

	case "verbose":
		m.viewMode = types.VerboseMode
		// Expand all messages
		for i := range m.messages {
			m.messages[i].IsCollapsed = false
		}
		m.state = types.NormalState
		return nil

	case "q", "quit":
		return tea.Quit

	default:
		m.state = types.NormalState
		return nil
	}
}

// generateID generates a unique ID for messages
func generateID(count int) string {
	if count == 0 {
		return "aa"
	}

	// Generate ID based on count (aa, ab, ac, ...)
	first := count / 26
	second := count % 26

	if first == 0 {
		return string(rune('a'+second)) + "a"
	}

	return string(rune('a'+first-1)) + string(rune('a'+second))
}
