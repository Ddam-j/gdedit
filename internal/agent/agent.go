package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gdedit/internal/config"
	"gdedit/internal/memo"
)

const (
	ModeMessage          = "message"
	ModeReplaceSelection = "replace_selection"
	ModeReplaceDocument  = "replace_document"
)

type Executor interface {
	Execute(ctx context.Context, req Request) (Response, error)
}

type Request struct {
	Command   string
	Action    string
	Kind      string
	Scope     string
	Tab       string
	Selection string
	Document  string
	Workspace string
}

type Response struct {
	Mode    string `json:"mode"`
	Content string `json:"content,omitempty"`
	Message string `json:"message"`
}

type Client struct {
	config     config.AgentConfig
	memoRoot   string
	baseURL    string
	httpClient *http.Client
}

func New(cfg config.AgentConfig, memoRoot string) (Executor, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.Status() != "ready" {
		return nil, nil
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider != "openai" {
		return nil, fmt.Errorf("unsupported edit-agent provider: %s", cfg.Provider)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &Client{
		config:   cfg,
		memoRoot: memoRoot,
		baseURL:  baseURL,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}, nil
}

func (c *Client) Execute(ctx context.Context, req Request) (Response, error) {
	apiKey := strings.TrimSpace(os.Getenv(c.config.APIKeyEnv))
	if apiKey == "" {
		return Response{}, fmt.Errorf("edit-agent API key env %q is not set", c.config.APIKeyEnv)
	}

	payload := chatCompletionsRequest{
		Model: c.config.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt()},
			{Role: "user", Content: c.userPrompt(req)},
		},
		Temperature: 0.2,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("edit-agent request failed: %s", strings.TrimSpace(string(respBody)))
	}

	var completions chatCompletionsResponse
	if err := json.Unmarshal(respBody, &completions); err != nil {
		return Response{}, err
	}
	if len(completions.Choices) == 0 {
		return Response{}, errors.New("edit-agent returned no choices")
	}

	content := strings.TrimSpace(completions.Choices[0].Message.Content)
	if content == "" {
		return Response{}, errors.New("edit-agent returned empty content")
	}

	return parseResponse(content)
}

func systemPrompt() string {
	return strings.TrimSpace(`You are the gdedit edit agent.
Respond with JSON only. Do not include markdown fences.
You must return an object with:
- mode: one of "message", "replace_selection", or "replace_document"
- message: short explanation for the user
- content: required only for replace_selection or replace_document

Rules:
- Stay scoped to the provided tab, command, scope, selection, and document.
- Prefer "message" for inspection or explanation requests.
- Use "replace_selection" for local edits when a selection is provided.
- Use "replace_document" only when the whole document clearly needs replacement.
- Never return extra keys or prose outside the JSON object.`)
}

func (c *Client) userPrompt(req Request) string {
	selection := req.Selection
	if strings.TrimSpace(selection) == "" {
		selection = "(none)"
	}
	memoContext := "(none)"
	if contextText, err := memo.LoadContext(c.memoRoot, req.Workspace); err == nil && strings.TrimSpace(contextText) != "" {
		memoContext = contextText
	}
	return fmt.Sprintf("command: %s\naction: %s\nkind: %s\ntab: %s\nscope: %s\nselection:\n%s\n\nmemos:\n%s\n\ndocument:\n%s\n", req.Command, req.Action, req.Kind, req.Tab, req.Scope, selection, memoContext, req.Document)
}

func parseResponse(content string) (Response, error) {
	jsonText := extractJSONObject(content)
	var response Response
	if err := json.Unmarshal([]byte(jsonText), &response); err != nil {
		return Response{}, err
	}

	response.Mode = strings.TrimSpace(response.Mode)
	response.Message = strings.TrimSpace(response.Message)
	if response.Mode == "" {
		response.Mode = ModeMessage
	}
	if response.Message == "" {
		response.Message = "Edit agent responded."
	}

	switch response.Mode {
	case ModeMessage:
		return response, nil
	case ModeReplaceSelection, ModeReplaceDocument:
		if response.Content == "" {
			return Response{}, errors.New("edit-agent response is missing content")
		}
		return response, nil
	default:
		return Response{}, fmt.Errorf("unsupported edit-agent mode: %s", response.Mode)
	}
}

func extractJSONObject(content string) string {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end >= start {
		return trimmed[start : end+1]
	}
	return trimmed
}

type chatCompletionsRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}
