package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/thebug/lab/eko/v3/pkg/types"
)

// renderMainView renders the main application view
func (m Model) renderMainView() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Ensure minimum dimensions to prevent panics
	if m.width < 20 {
		m.width = 20
	}
	if m.height < 10 {
		m.height = 10
	}

	header := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(amoblackColor).
		Render(fmt.Sprintf("EKO - Model: %s | Messages: %d", m.modelName, len(m.messages)))

	inputView := ""
	if m.state == types.InsertState || m.state == types.CommandState {
		inputView = m.input.View()
	} else {
		inputView = "Press 'i' to type, ':' for commands\n h/j for up/down\nq to quit"
	}

	// Style the input with rounded corners and center alignment
	inputWidth := m.width - 4
	if inputWidth < 10 {
		inputWidth = 10
	}
	inputLine := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderTop(true).
		BorderForeground(accentColor).
		Padding(1, 2).
		Margin(1, 0).
		Align(lipgloss.Center).
		Width(inputWidth).
		Render(inputView)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		m.viewport.View(),
		inputLine,
	)
}

// renderMessages renders all messages
func (m Model) renderMessages() string {
	var b strings.Builder

	// Ensure minimum width to prevent panics
	contentWidth := m.width
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Calculate message width - use about 60% of screen width for messages
	messageWidth := int(float64(contentWidth) * 0.6)
	if messageWidth < 20 {
		messageWidth = 20
	}

	for i, msg := range m.messages {
		// Add small breathing room between different message types
		if i > 0 {
			prevMsg := m.messages[i-1]
			if prevMsg.Role != msg.Role {
				// Add a subtle separator between user and assistant messages
				b.WriteString("\n")
			}
		}

		// Content (with TLDR handling)
		content := msg.Content
		if m.viewMode == types.TLDRMode && msg.IsCollapsed && len(content) > 100 {
			content = content[:100] + "..."
		}

		// Show spinner if this is the last message and still processing
		if msg.Role == "assistant" && msg.Content == "" && len(m.messages) > 0 &&
			msg.ID == m.messages[len(m.messages)-1].ID && m.streaming {
			content = m.spinner.View() + " " + content
		}

		// Time and divider (divider only used when metadata will be shown)
		timeStr := msg.Timestamp.Format("15:04:05")

		var cardContent string
		if msg.Role == "assistant" {
			divider := lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")).Render(strings.Repeat("─", messageWidth-4))
			metadata := fmt.Sprintf("ID: %s | Time: %s", msg.ID, timeStr)
			cardContent = fmt.Sprintf("%s\n%s\n%s", content, divider, metadata)
		} else {
			// User messages: white text only, no divider, no metadata
			textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
			cardContent = textStyle.Render(content)
		}

		// Create message card with rounded corners and gray borders
		var messageStyle lipgloss.Style

		if msg.Role == "user" {
			// User messages: positioned at extreme right
			messageStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(amoblackColor).
				Padding(0, 1).
				Margin(0, 0, 0, 0).
				Width(messageWidth).
				Align(lipgloss.Right)

			// Position at extreme right by adding left margin
			leftMargin := contentWidth - messageWidth - 2
			if leftMargin < 0 {
				leftMargin = 0
			}
			messageStyle = messageStyle.MarginLeft(leftMargin)
		} else {
			// Assistant messages: positioned at extreme left
			messageStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(amoblackColor).
				Padding(0, 1).
				Margin(0, 0, 0, 0).
				Width(messageWidth).
				Align(lipgloss.Left)
		}

		// Render the message card
		messageCard := messageStyle.Render(cardContent)
		b.WriteString(messageCard)
		b.WriteString("\n")
	}

	return b.String()
}

// renderModelList renders the model selection list
func (m Model) renderModelList() string {
	if len(m.modelList) == 0 {
		return "Loading models..."
	}

	var b strings.Builder
	b.WriteString("Select a model (j/k to navigate, enter to select, esc to cancel):\n\n")

	for i, model := range m.modelList {
		if i == m.selectedIdx {
			b.WriteString("> " + lipgloss.NewStyle().Foreground(accentColor).Render(model) + "\n")
		} else {
			b.WriteString("  " + model + "\n")
		}
	}

	return b.String()
}
