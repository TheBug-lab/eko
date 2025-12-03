package ui

import (
	"fmt"
	"strings"
	"time"

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

	// Add status line for yank mode
	statusLine := ""
	if m.state == types.YankCodeState {
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFF00")). // Yellow color for yank mode
			Render("[YANK MODE] Enter code block ID: " + m.yankInput)
	} else if m.yankStatus != "" && time.Since(m.yankStatusTimer) < 3*time.Second {
		// Show yank status for 3 seconds
		var style lipgloss.Style
		if strings.HasPrefix(m.yankStatus, "✔") {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")) // Green for success
		} else {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")) // Red for error
		}
		statusLine = style.Render(m.yankStatus)
	}

	inputView := ""
	if m.state == types.InsertState || m.state == types.CommandState {
		inputView = m.input.View()
	} else if m.state == types.YankCodeState {
		// Don't show anything in input area for yank mode
		inputView = ""
	} else {
		inputView = "press 'i' for insert mode\n q for quit"
	}

	// Style the input with rounded corners and center alignment
	// Make input width adaptive to screen size (use 80% of screen width, min 30, max 120)
	inputWidth := int(float64(m.width) * 0.8)
	if inputWidth < 30 {
		inputWidth = 30
	} else if inputWidth > 120 {
		inputWidth = 120
	}

	// Style for 2-line height input with center alignment
	inputLine := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderTop(true).
		BorderForeground(accentColor).
		Padding(0, 1). // Minimal padding for 2-line height
		Margin(0, 0).  // No margin
		Align(lipgloss.Center).
		Width(inputWidth).
		Height(2). // Set to 2 lines height
		Render(inputView)

	// Center everything on the screen
	var content string
	if statusLine != "" {
		content = lipgloss.JoinVertical(
			lipgloss.Center, // Center align vertically
			header,
			statusLine,
			m.viewport.View(),
			inputLine,
		)
	} else {
		content = lipgloss.JoinVertical(
			lipgloss.Center, // Center align vertically
			header,
			m.viewport.View(),
			inputLine,
		)
	}

	// Add IMAGE tag if in image mode
	if m.isImageMode {
		// Create a 1-line tag: [ image ] to save height
		bracketStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#fe3f01"))
		textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#800000"))
		
		imageTag := fmt.Sprintf("%s%s%s", 
			bracketStyle.Render("["),
			textStyle.Render("image"),
			bracketStyle.Render("]"),
		)
		
		// Add queue count
		queueText := fmt.Sprintf("%d ", m.queueCount)
		queueStyled := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render(queueText)
		
		fullTag := lipgloss.JoinHorizontal(lipgloss.Center, queueStyled, imageTag)
		
		// Let's go with a footer row.
		return lipgloss.JoinVertical(
			lipgloss.Top,
			lipgloss.NewStyle().
				Align(lipgloss.Center).
				Width(m.width).
				Height(m.height-1).
				Render(content),
			lipgloss.NewStyle().
				Width(m.width).
				Align(lipgloss.Right).
				Render(fullTag),
		)
	}

	// Center the entire content horizontally on the screen
	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(m.width).
		Height(m.height).
		Render(content)
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

		// Content (with TLDR handling and code block processing)
		content := msg.Content
		if m.viewMode == types.TLDRMode && msg.IsCollapsed && len(content) > 100 {
			content = content[:100] + "..."
		} else if msg.Role == "assistant" {
			// Process code blocks for assistant messages
			content = ReplaceCodeBlocksInContent(content, msg.ID, messageWidth)
		}

		// Show spinner if this is the last message and still processing
		if msg.Role == "assistant" && len(m.messages) > 0 &&
			msg.ID == m.messages[len(m.messages)-1].ID && m.isThinking {
			if m.isImageMode {
				// Custom thin progress bar
				barWidth := 30
				
				// Calculate percentages first
				var displayPct float64 = m.progressPct // Default to total
				
				// Node progress - calculate percentage within current node
				nodePctStr := ""
				if m.nodeProgress != "" {
					// Parse "5/9" format to calculate node percentage
					var val, max int
					if _, err := fmt.Sscanf(m.nodeProgress, "%d/%d", &val, &max); err == nil && max > 0 {
						displayPct = float64(val) / float64(max)
						nodePct := int(displayPct * 100)
						nodePctStr = fmt.Sprintf("%d%% of %s", nodePct, m.nodeProgress)
					}
				}

				filledWidth := int(displayPct * float64(barWidth))
				if filledWidth > barWidth {
					filledWidth = barWidth
				}
				emptyWidth := barWidth - filledWidth
				
				// Use thin characters
				filledChar := "━"
				emptyChar := "─"
				
				filled := strings.Repeat(filledChar, filledWidth)
				empty := strings.Repeat(emptyChar, emptyWidth)
				
				// Color the filled part with orange
				filledStyled := lipgloss.NewStyle().Foreground(lipgloss.Color("#fe3f01")).Render(filled)
				emptyStyled := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render(empty)
				
				// Percentage - show node% only
				
				// Time calculation: Elapsed / Total
				elapsed := m.elapsedTime.Round(time.Second)
				totalStr := "?"
				
				if m.progressPct > 0.01 {
					totalEstimated := time.Duration(float64(m.elapsedTime) / m.progressPct).Round(time.Second)
					totalStr = totalEstimated.String()
				}
				
				timeStr := fmt.Sprintf(" | %s/%s", elapsed, totalStr)
				
				// Combine everything into one line
				infoText := fmt.Sprintf("%s%s", nodePctStr, timeStr)
				infoStyled := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render(infoText)
				
				content = fmt.Sprintf("%s%s\n%s", filledStyled, emptyStyled, infoStyled)
			} else if msg.Content == "" {
				content = m.spinner.View() + " AI is thinking..."
			} else {
				// Show spinner while content is being streamed
				content = content + " " + m.spinner.View()
			}
		}

		// Time and divider (divider only used when metadata will be shown)
		timeStr := msg.Timestamp.Format("15:04:05")

		var cardContent string
		if msg.Role == "assistant" {
			divider := lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")).Render(strings.Repeat("─", messageWidth-4))
			metadata := fmt.Sprintf("%s | %s", msg.ID, timeStr)
			cardContent = fmt.Sprintf("%s\n%s\n%s", content, divider, metadata)
		} else {
			// User messages: white text only, no divider, no metadata
			textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
			cardContent = textStyle.Render(content)
		}

		// Create message card with no borders
		var messageStyle lipgloss.Style

		if msg.Role == "user" {
			// User messages: positioned at extreme right
			messageStyle = lipgloss.NewStyle().
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
