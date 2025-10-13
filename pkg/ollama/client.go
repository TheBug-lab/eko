package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/thebug/lab/eko/v3/pkg/types"
)

// Client handles communication with Ollama API
type Client struct {
	BaseURL string
	Client  *http.Client
}

// Request represents an Ollama API request
type Request struct {
	Model    string         `json:"model"`
	Messages []types.Message `json:"messages"`
	Stream   bool           `json:"stream"`
}

// Response represents an Ollama API response
type Response struct {
	Model     string        `json:"model"`
	Message   types.Message `json:"message"`
	Done      bool          `json:"done"`
	CreatedAt string        `json:"created_at"`
}

// ModelInfo represents a model from Ollama
type ModelInfo struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
	Digest     string    `json:"digest"`
	Details    struct {
		Format            string `json:"format"`
		Family            string `json:"family"`
		Families          []string `json:"families"`
		ParameterSize     string `json:"parameter_size"`
		QuantizationLevel string `json:"quantization_level"`
	} `json:"details"`
}

// NewClient creates a new Ollama client
func NewClient() *Client {
	return &Client{
		BaseURL: "http://localhost:11434",
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchModels fetches available models from Ollama
func (c *Client) FetchModels() tea.Cmd {
	return func() tea.Msg {
		resp, err := c.Client.Get(c.BaseURL + "/api/tags")
		if err != nil {
			return types.ModelsLoadedMsg{Models: nil, Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return types.ModelsLoadedMsg{Models: nil, Err: fmt.Errorf("ollama API returned status %d", resp.StatusCode)}
		}

		var response struct {
			Models []ModelInfo `json:"models"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return types.ModelsLoadedMsg{Models: nil, Err: err}
		}

		models := make([]string, len(response.Models))
		for i, model := range response.Models {
			models[i] = model.Name
		}

		return types.ModelsLoadedMsg{Models: models, Err: nil}
	}
}

// StreamChat streams a chat response from Ollama
func (c *Client) StreamChat(model string, messages []types.Message, onToken func(string, bool)) error {
	req := Request{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := c.Client.Post(c.BaseURL+"/api/chat", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama API returned status %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	for {
		var response Response
		if err := decoder.Decode(&response); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode response: %v", err)
		}

		onToken(response.Message.Content, response.Done)

		if response.Done {
			break
		}
	}

	return nil
}

// StreamChatRealtime streams a chat response from Ollama with real-time updates via channel
func (c *Client) StreamChatRealtime(model string, messages []types.Message, msgChan chan<- tea.Msg, messageID string) tea.Cmd {
	return func() tea.Msg {
		req := Request{
			Model:    model,
			Messages: messages,
			Stream:   true,
		}

		jsonData, err := json.Marshal(req)
		if err != nil {
			msgChan <- types.StreamErrorMsg{ID: messageID, Error: fmt.Sprintf("failed to marshal request: %v", err)}
			return nil
		}

		resp, err := c.Client.Post(c.BaseURL+"/api/chat", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			msgChan <- types.StreamErrorMsg{ID: messageID, Error: fmt.Sprintf("failed to make request: %v", err)}
			return nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			msgChan <- types.StreamErrorMsg{ID: messageID, Error: fmt.Sprintf("ollama API returned status %d", resp.StatusCode)}
			return nil
		}

		decoder := json.NewDecoder(resp.Body)
		for {
			var response Response
			if err := decoder.Decode(&response); err != nil {
				if err == io.EOF {
					break
				}
				msgChan <- types.StreamErrorMsg{ID: messageID, Error: fmt.Sprintf("failed to decode response: %v", err)}
				return nil
			}

			// Send token immediately via channel
			if response.Message.Content != "" {
				msgChan <- types.TokenMsg{ID: messageID, Token: response.Message.Content}
			}

			if response.Done {
				msgChan <- types.GenerationDoneMsg{ID: messageID}
				break
			}
		}

		return nil
	}
}
