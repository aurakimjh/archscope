package aiinterpretation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type LocalLlmConfig struct {
	Enabled        bool
	BaseURL        string
	Model          string
	TimeoutSeconds int
}

type OllamaClient struct {
	Config LocalLlmConfig
	Client *http.Client
}

func (c OllamaClient) Execute(ctx context.Context, prompt PromptPayload, registry *EvidenceRegistry) (map[string]any, error) {
	if !c.Config.Enabled {
		return nil, fmt.Errorf("local LLM execution is disabled")
	}
	baseURL := c.Config.BaseURL
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	if err := validateLocalURL(baseURL); err != nil {
		return nil, err
	}
	model := c.Config.Model
	if model == "" {
		model = "llama3.1"
	}
	timeout := c.Config.TimeoutSeconds
	if timeout <= 0 {
		timeout = 60
	}
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: time.Duration(timeout) * time.Second}
	}
	body, err := json.Marshal(map[string]any{
		"model":  model,
		"prompt": prompt.System + "\n\n" + prompt.User,
		"stream": false,
		"format": "json",
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama returned HTTP %d", resp.StatusCode)
	}
	var envelope struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	var interpretation map[string]any
	if err := json.Unmarshal([]byte(envelope.Response), &interpretation); err != nil {
		return nil, fmt.Errorf("ollama response was not valid interpretation JSON: %w", err)
	}
	interpretation["provider"] = "ollama"
	if _, ok := interpretation["model"]; !ok {
		interpretation["model"] = model
	}
	if _, ok := interpretation["prompt_version"]; !ok {
		interpretation["prompt_version"] = prompt.Version
	}
	return AiFindingValidator{
		Registry:              registry,
		RequireEvidenceQuotes: true,
	}.ValidateInterpretation(interpretation)
}

func validateLocalURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("local LLM URL must use http or https")
	}
	host := parsed.Hostname()
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return fmt.Errorf("local LLM URL must point to localhost")
	}
	return nil
}
