package main

import (
	"context"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"
)

// OpenRouterClientConfig represents the configuration for the OpenRouter API client.
type OpenRouterClientConfig struct {
	AuthToken  string
	BaseURL    string
	HTTPClient *http.Client
	User       string
}

// DefaultOpenRouterConfig returns the default configuration for the OpenRouter API client.
func DefaultOpenRouterConfig(authToken string) OpenRouterClientConfig {
	return OpenRouterClientConfig{
		AuthToken:  authToken,
		BaseURL:    "https://openrouter.ai/api/v1",
		HTTPClient: &http.Client{},
	}
}

// createOpenRouterStream creates a stream for OpenRouter chat completion.
func (m *Mods) createOpenRouterStream(content string, ccfg openai.ClientConfig, mod Model) tea.Msg {
	cfg := m.Config

	// OpenRouter specific parameters - add HTTP headers
	httpHeader := make(http.Header)
	httpHeader.Set("HTTP-Referer", "https://github.com/charmbracelet/mods")
	httpHeader.Set("X-Title", "Mods CLI")

	// Create custom transport
	transport := http.DefaultTransport
	if httpClient, ok := ccfg.HTTPClient.(*http.Client); ok && httpClient.Transport != nil {
		transport = httpClient.Transport
	}

	// Create a new HTTP client with our custom transport
	ccfg.HTTPClient = &http.Client{
		Transport: &headerTransport{
			transport: transport,
			headers:   httpHeader,
		},
		Timeout: time.Second * 30,
	}

	client := openai.NewClientWithConfig(ccfg)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelRequest = cancel

	if err := m.setupStreamContext(content, mod); err != nil {
		return err
	}

	req := openai.ChatCompletionRequest{
		Model:       mod.Name,
		Messages:    m.messages,
		Stream:      true,
		User:        cfg.User,
		Temperature: noOmitFloat(cfg.Temperature),
		TopP:        noOmitFloat(cfg.TopP),
		Stop:        cfg.Stop,
		MaxTokens:   cfg.MaxTokens,
	}

	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return m.handleRequestError(err, mod, content)
	}

	return m.receiveCompletionStreamCmd(completionOutput{stream: stream})()
}

// headerTransport is a custom transport that adds headers to requests.
type headerTransport struct {
	transport http.RoundTripper
	headers   http.Header
}

// RoundTrip implements the http.RoundTripper interface.
func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Use the default transport if none is specified
	if t.transport == nil {
		t.transport = http.DefaultTransport
	}

	// Clone the request to avoid modifying the original
	clonedReq := req.Clone(req.Context())

	// Add headers
	for key, values := range t.headers {
		for _, value := range values {
			clonedReq.Header.Add(key, value)
		}
	}

	// Call the underlying transport
	return t.transport.RoundTrip(clonedReq)
}
