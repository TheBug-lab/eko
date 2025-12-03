package comfyui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Debug logging
func logDebug(format string, v ...interface{}) {
	f, err := os.OpenFile("eko-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	timestamp := time.Now().Format("15:04:05.000")
	fmt.Fprintf(f, timestamp+" "+format+"\n", v...)
}

type Client struct {
	BaseURL string
	ClientID string
}

type ProgressUpdate struct {
	Value          int
	Max            int
	NodeID         string
	Message        string
	Percent        float64
	ElapsedTime    time.Duration
	QueueRemaining int
}

func NewClient(baseURL string) *Client {
	// Generate a simple random client ID
	rand.Seed(time.Now().UnixNano())
	clientID := fmt.Sprintf("eko-%d", rand.Int63())
	
	return &Client{
		BaseURL:  baseURL,
		ClientID: clientID,
	}
}

// GenerateImage sends a prompt to ComfyUI and waits for the result
func (c *Client) GenerateImage(workflowJSON []byte, prompt string, progressChan chan<- ProgressUpdate) (string, error) {
	// 1. Parse the workflow JSON
	var workflow map[string]interface{}
	if err := json.Unmarshal(workflowJSON, &workflow); err != nil {
		return "", fmt.Errorf("failed to parse workflow JSON: %w", err)
	}

	// Check for aspect ratio override in prompt
	// Pattern: ar-<width>:<height>
	arRegex := regexp.MustCompile(`ar-(\d+):(\d+)`)
	matches := arRegex.FindStringSubmatch(prompt)
	
	var overrideWidth, overrideHeight int
	if len(matches) == 3 {
		// Found override
		w, err1 := strconv.Atoi(matches[1])
		h, err2 := strconv.Atoi(matches[2])
		if err1 == nil && err2 == nil {
			overrideWidth = w
			overrideHeight = h
			// Remove the tag from prompt
			prompt = strings.TrimSpace(arRegex.ReplaceAllString(prompt, ""))
		}
	}

	// 2. Inject the prompt into the workflow
	// Heuristic: Find the best CLIPTextEncode node
	var positiveNodeID string
	var negativeNodeID string
	var lastTextNodeID string
	
	for nodeID, node := range workflow {
		nodeMap, ok := node.(map[string]interface{})
		if !ok {
			continue
		}
		classType, ok := nodeMap["class_type"].(string)
		if !ok {
			continue
		}

		// Randomize seed in KSampler
		if classType == "KSampler" || classType == "KSamplerAdvanced" {
			inputs, ok := nodeMap["inputs"].(map[string]interface{})
			if ok {
				if _, hasSeed := inputs["seed"]; hasSeed {
					// Generate a random seed (ComfyUI uses large integers)
					inputs["seed"] = rand.Int63()
					logDebug("Randomized seed for node %s", nodeID)
				}
			}
		}

		if classType == "CLIPTextEncode" || classType == "ShowText" || classType == "PrimitiveString" {
			// Check metadata
			if meta, ok := nodeMap["_meta"].(map[string]interface{}); ok {
				if title, ok := meta["title"].(string); ok {
					lowerTitle := strings.ToLower(title)
					if strings.Contains(lowerTitle, "positive") {
						positiveNodeID = nodeID
					} else if strings.Contains(lowerTitle, "negative") {
						negativeNodeID = nodeID
					}
				}
			}
			lastTextNodeID = nodeID
		}
		
		// Override dimensions if found
		// Support both EmptyLatentImage and EmptySD3LatentImage
		if overrideWidth > 0 && overrideHeight > 0 && (classType == "EmptyLatentImage" || classType == "EmptySD3LatentImage") {
			inputs, ok := nodeMap["inputs"].(map[string]interface{})
			if ok {
				if _, hasWidth := inputs["width"]; hasWidth {
					inputs["width"] = overrideWidth
				}
				if _, hasHeight := inputs["height"]; hasHeight {
					inputs["height"] = overrideHeight
				}
			}
		}
	}
	
	// Decide which node to inject into
	targetNodeID := ""
	if positiveNodeID != "" {
		targetNodeID = positiveNodeID
	} else if lastTextNodeID != "" && lastTextNodeID != negativeNodeID {
		// If we didn't find a positive one, but found a text node that isn't explicitly negative
		targetNodeID = lastTextNodeID
	}
	
	if targetNodeID != "" {
		if node, ok := workflow[targetNodeID].(map[string]interface{}); ok {
			if inputs, ok := node["inputs"].(map[string]interface{}); ok {
				inputs["text"] = prompt
				logDebug("Injected prompt into node %s", targetNodeID)
			}
		}
	} else {
		logDebug("WARNING: Could not find a suitable node to inject prompt!")
		// Fallback: Inject into ALL text nodes that aren't negative?
		// Or just fail?
	}

	// 3. Connect to WebSocket
	wsURL := strings.Replace(c.BaseURL, "http", "ws", 1) + "/ws?clientId=" + c.ClientID
	logDebug("Connecting to WebSocket: %s", wsURL)
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	defer ws.Close()

	// 4. Send to ComfyUI
	payload := map[string]interface{}{
		"prompt":    workflow,
		"client_id": c.ClientID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	logDebug("Sending prompt to %s/prompt", c.BaseURL)
	resp, err := http.Post(fmt.Sprintf("%s/prompt", c.BaseURL), "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to send request to ComfyUI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ComfyUI returned error: %s", string(body))
	}

	var promptResp struct {
		PromptID string `json:"prompt_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&promptResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	promptID := promptResp.PromptID
	logDebug("Prompt ID: %s", promptID)
	startTime := time.Now()
	
	// Track execution state
	totalNodes := len(workflow)
	logDebug("Total nodes in workflow: %d", totalNodes)
	executedNodes := make(map[string]bool)
	var generatedImages []string

	if progressChan != nil {
		progressChan <- ProgressUpdate{
			Message:     "Queued...",
			ElapsedTime: 0,
		}
	}

	// 5. Listen for WebSocket messages
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			return "", fmt.Errorf("websocket read error: %w", err)
		}

		// logDebug("Received WS message: %s", string(message))

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		msgType, _ := msg["type"].(string)
		data, _ := msg["data"].(map[string]interface{})

		// Log raw message for debugging
		// logDebug("WS Type: %s", msgType)

		switch msgType {
		case "status":
			if progressChan != nil {
				status, _ := data["status"].(map[string]interface{})
				if status != nil {
					execInfo, _ := status["exec_info"].(map[string]interface{})
					if execInfo != nil {
						queueRemaining, _ := execInfo["queue_remaining"].(float64)
						logDebug("Queue remaining: %v", queueRemaining)
						progressChan <- ProgressUpdate{
							Message:        fmt.Sprintf("Queue position: %d", int(queueRemaining)),
							ElapsedTime:    time.Since(startTime),
							QueueRemaining: int(queueRemaining),
						}
					}
				}
			}
		case "execution_start":
			pid, _ := data["prompt_id"].(string)
			logDebug("Execution start event for %s (we want %s)", pid, promptID)
			if pid == promptID {
				if progressChan != nil {
					progressChan <- ProgressUpdate{
						Message:     "Processing started...",
						ElapsedTime: time.Since(startTime),
					}
				}
			}
		case "executing":
			node := data["node"]
			if node == nil {
				// Execution finished!
				if len(generatedImages) > 0 {
					return fmt.Sprintf("Image(s) generated: %s", strings.Join(generatedImages, ", ")), nil
				}
				return "Generation complete", nil
			} else {
				// Check prompt_id if available, but be permissive
				pid, _ := data["prompt_id"].(string)
				if pid != "" && pid != promptID {
					continue
				}

				var nodeID string
				switch v := node.(type) {
				case string:
					nodeID = v
				case float64:
					nodeID = fmt.Sprintf("%.0f", v)
				default:
					nodeID = fmt.Sprintf("%v", v)
				}
				
				logDebug("Executing node: %s", nodeID)
				if progressChan != nil {
					// Calculate total progress
					executedCount := len(executedNodes)
					totalPct := float64(executedCount) / float64(totalNodes)
					
					progressChan <- ProgressUpdate{
						Message:     fmt.Sprintf("Executing node %s", nodeID),
						Percent:     totalPct,
						ElapsedTime: time.Since(startTime),
					}
				}
			}
		case "progress":
			pid, _ := data["prompt_id"].(string)
			// logDebug("Progress event for %s: %v", pid, data)
			if pid == promptID {
				val, _ := data["value"].(float64)
				max, _ := data["max"].(float64)
				
				logDebug("Progress: %v/%v", val, max)
				if progressChan != nil && max > 0 {
					// Calculate weighted progress
					// Base progress from executed nodes
					executedCount := len(executedNodes)
					
					// Add current node progress (scaled by 1/totalNodes)
					nodeProgress := float64(val) / float64(max)
					totalPct := (float64(executedCount) + nodeProgress) / float64(totalNodes)
					
					progressChan <- ProgressUpdate{
						Value:       int(val),
						Max:         int(max),
						Percent:     totalPct,
						ElapsedTime: time.Since(startTime),
					}
				}
			}
		case "executed":
			pid, _ := data["prompt_id"].(string)
			if pid == promptID {
				// Handle node ID which might be string or float64
				var nodeID string
				if n, ok := data["node"].(string); ok {
					nodeID = n
				} else if n, ok := data["node"].(float64); ok {
					nodeID = fmt.Sprintf("%.0f", n)
				}
				
				logDebug("Node %s executed", nodeID)
				if nodeID != "" {
					executedNodes[nodeID] = true
				}
				
				// Check for output images
				if output, ok := data["output"].(map[string]interface{}); ok {
					for _, nodeOutput := range output {
						if images, ok := nodeOutput.([]interface{}); ok {
							for _, img := range images {
								if imgMap, ok := img.(map[string]interface{}); ok {
									filename, okName := imgMap["filename"].(string)
									subfolder, _ := imgMap["subfolder"].(string)
									imgType, _ := imgMap["type"].(string)
									
									if okName {
										// Download the image
										downloadedFile, err := c.downloadImage(filename, subfolder, imgType)
										if err == nil {
											generatedImages = append(generatedImages, downloadedFile)
										} else {
											generatedImages = append(generatedImages, fmt.Sprintf("%s (failed: %v)", filename, err))
										}
									}
								}
							}
						}
					}
				}
			}
		case "execution_error":
			pid, _ := data["prompt_id"].(string)
			if pid == promptID {
				return "", fmt.Errorf("execution error: %v", data["exception_message"])
			}
		}
	}
}

// downloadImage downloads an image from ComfyUI to the current directory
func (c *Client) downloadImage(filename, subfolder, imgType string) (string, error) {
	// Construct URL
	params := url.Values{}
	params.Add("filename", filename)
	if subfolder != "" {
		params.Add("subfolder", subfolder)
	}
	if imgType != "" {
		params.Add("type", imgType)
	}
	
	imgURL := fmt.Sprintf("%s/view?%s", c.BaseURL, params.Encode())
	
	resp, err := http.Get(imgURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status code %d", resp.StatusCode)
	}
	
	// Save to current directory
	// Generate new filename: eko-img-<timestamp>
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".png"
	}
	
	timestamp := time.Now().Format("20060102-150405")
	newFilename := fmt.Sprintf("eko-img-%s%s", timestamp, ext)
	
	// Handle collision
	counter := 1
	for {
		if _, err := os.Stat(newFilename); os.IsNotExist(err) {
			break
		}
		newFilename = fmt.Sprintf("eko-img-%s-%d%s", timestamp, counter, ext)
		counter++
	}

	outFile, err := os.Create(newFilename)
	if err != nil {
		return "", err
	}
	defer outFile.Close()
	
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", err
	}
	
	// Return absolute path for clarity
	absPath, err := filepath.Abs(newFilename)
	if err != nil {
		return newFilename, nil
	}
	return absPath, nil
}

// GetQueueRemaining fetches the current number of items in the queue
func (c *Client) GetQueueRemaining() (int, error) {
	resp, err := http.Get(c.BaseURL + "/prompt")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	
	var result struct {
		ExecInfo struct {
			QueueRemaining int `json:"queue_remaining"`
		} `json:"exec_info"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	
	return result.ExecInfo.QueueRemaining, nil
}
