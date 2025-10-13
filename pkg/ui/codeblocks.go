package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/thebug/lab/eko/v3/pkg/types"
)

// Global code block storage
var codeBlocks = make(map[string]types.CodeBlock)

// codeBlockRegex matches markdown code blocks with optional language
var codeBlockRegex = regexp.MustCompile("```(\\w*)\\n([\\s\\S]*?)```")

// generateCodeBlockID creates a unique ID for a code block using parentID+letter format
func generateCodeBlockID(messageID string, index int) string {
	// Convert index to letter (a, b, c, ...)
	letter := string(rune('a' + index))
	return messageID + letter
}

// RenderCodeBlock renders a code block with gray background and ID in bottom right
func RenderCodeBlock(block types.CodeBlock, width int) string {
	// Ensure minimum width to prevent crashes
	if width < 20 {
		width = 80
	}

	// Apply syntax highlighting
	highlightedContent := highlightCode(block.Content, block.Language)

	// Language display
	languageDisplay := block.Language
	if languageDisplay == "" {
		languageDisplay = "code"
	}

	// Create gray background style that covers the entire block
	grayStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#0f0f0f")).
		Foreground(lipgloss.Color("#fe3f01")).
		Padding(1, 2).
		Margin(0, 0, 1, 0).
		Width(width - 4) // Set explicit width to ensure full coverage

	// Add header with language
	header := fmt.Sprintf("%s code", languageDisplay)

	// Split content into lines for processing
	lines := strings.Split(highlightedContent, "\n")

	// Add ID to bottom right corner
	if len(lines) > 0 {
		lastLine := lines[len(lines)-1]
		// Calculate padding needed to right-align the ID
		idText := "[" + block.ID + "]"
		// Account for padding and width
		availableWidth := width - 4 - 4 // width - padding - some buffer
		paddingNeeded := availableWidth - len(lastLine) - len(idText)
		if paddingNeeded < 0 {
			paddingNeeded = 0
		}
		lines[len(lines)-1] = lastLine + strings.Repeat(" ", paddingNeeded) + idText
	}

	// Combine header and content
	content := header + "\n" + strings.Join(lines, "\n")

	return grayStyle.Render(content)
}

// highlightCode applies basic syntax highlighting to code content
func highlightCode(content, language string) string {
	// Return plain text if content is empty
	if content == "" {
		return content
	}

	// For now, return content as is with minimal highlighting
	// We can add more sophisticated highlighting later
	return content
}

// ReplaceCodeBlocksInContent replaces code blocks in content with rendered versions
func ReplaceCodeBlocksInContent(content string, messageID string, width int) string {
	// Ensure minimum width to prevent crashes
	if width < 20 {
		width = 80
	}

	// Find all code blocks using regex
	matches := codeBlockRegex.FindAllStringSubmatch(content, -1)

	if len(matches) == 0 {
		return content
	}

	// Process each match and replace it
	for i, match := range matches {
		if len(match) >= 3 {
			language := strings.TrimSpace(match[1])
			codeContent := strings.TrimSpace(match[2])

			// Generate unique ID
			blockID := generateCodeBlockID(messageID, i)

			// Create code block
			block := types.CodeBlock{
				ID:        blockID,
				Language:  language,
				Content:   codeContent,
				MessageID: messageID,
			}

			// Store in global map
			codeBlocks[blockID] = block

			// Render the block
			renderedBlock := RenderCodeBlock(block, width)

			// Replace the original code block
			originalBlock := match[0] // The full match including ```
			content = strings.Replace(content, originalBlock, renderedBlock, 1)
		}
	}

	return content
}

// GetCodeBlock retrieves a code block by ID
func GetCodeBlock(blockID string) (types.CodeBlock, bool) {
	block, exists := codeBlocks[blockID]
	return block, exists
}

// GetAllCodeBlocks returns all code blocks for a message
func GetAllCodeBlocks(messageID string) []types.CodeBlock {
	var blocks []types.CodeBlock
	for _, block := range codeBlocks {
		if block.MessageID == messageID {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// ListAllCodeBlocks returns all code block IDs for debugging
func ListAllCodeBlocks() []string {
	var ids []string
	for id := range codeBlocks {
		ids = append(ids, id)
	}
	return ids
}
