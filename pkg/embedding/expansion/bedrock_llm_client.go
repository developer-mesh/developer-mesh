package expansion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// BedrockLLMClient implements LLMClient using AWS Bedrock
type BedrockLLMClient struct {
	client  *bedrockruntime.Client
	modelID string
}

// NewBedrockLLMClient creates a new Bedrock LLM client
func NewBedrockLLMClient(region, modelID string) (*BedrockLLMClient, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := bedrockruntime.NewFromConfig(cfg)

	// Default to Claude if no model specified
	if modelID == "" {
		modelID = "anthropic.claude-v2"
	}

	return &BedrockLLMClient{
		client:  client,
		modelID: modelID,
	}, nil
}

// Complete implements the LLMClient interface
func (b *BedrockLLMClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	// Format prompt based on model
	var requestBody []byte
	var err error

	switch {
	case contains(b.modelID, "anthropic.claude"):
		requestBody, err = b.formatClaudeRequest(req)
	case contains(b.modelID, "amazon.titan"):
		requestBody, err = b.formatTitanRequest(req)
	default:
		return nil, fmt.Errorf("unsupported model: %s", b.modelID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to format request: %w", err)
	}

	// Invoke model
	resp, err := b.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(b.modelID),
		Body:        requestBody,
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to invoke model: %w", err)
	}

	// Parse response based on model
	switch {
	case contains(b.modelID, "anthropic.claude"):
		return b.parseClaudeResponse(resp.Body)
	case contains(b.modelID, "amazon.titan"):
		return b.parseTitanResponse(resp.Body)
	default:
		return nil, fmt.Errorf("unsupported model: %s", b.modelID)
	}
}

// formatClaudeRequest formats a request for Claude models
func (b *BedrockLLMClient) formatClaudeRequest(req CompletionRequest) ([]byte, error) {
	var prompt string
	if req.SystemPrompt != "" {
		prompt = fmt.Sprintf("System: %s\n\nHuman: %s\n\nAssistant:", req.SystemPrompt, req.Prompt)
	} else {
		prompt = fmt.Sprintf("Human: %s\n\nAssistant:", req.Prompt)
	}

	claudeReq := map[string]interface{}{
		"prompt":               prompt,
		"max_tokens_to_sample": req.MaxTokens,
		"temperature":          req.Temperature,
		"top_p":                0.9,
		"stop_sequences":       []string{"\n\nHuman:"},
	}

	return json.Marshal(claudeReq)
}

// formatTitanRequest formats a request for Titan models
func (b *BedrockLLMClient) formatTitanRequest(req CompletionRequest) ([]byte, error) {
	titanReq := map[string]interface{}{
		"inputText": req.Prompt,
		"textGenerationConfig": map[string]interface{}{
			"maxTokenCount": req.MaxTokens,
			"temperature":   req.Temperature,
			"topP":          0.9,
			"stopSequences": []string{},
		},
	}

	return json.Marshal(titanReq)
}

// parseClaudeResponse parses a Claude model response
func (b *BedrockLLMClient) parseClaudeResponse(body []byte) (*CompletionResponse, error) {
	var resp struct {
		Completion string `json:"completion"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Claude response: %w", err)
	}

	// Estimate tokens (rough approximation)
	tokens := len(resp.Completion) / 4

	return &CompletionResponse{
		Text:   resp.Completion,
		Tokens: tokens,
	}, nil
}

// parseTitanResponse parses a Titan model response
func (b *BedrockLLMClient) parseTitanResponse(body []byte) (*CompletionResponse, error) {
	var resp struct {
		Results []struct {
			OutputText       string `json:"outputText"`
			CompletionReason string `json:"completionReason"`
			TokenCount       int    `json:"tokenCount"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Titan response: %w", err)
	}

	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("no results in Titan response")
	}

	result := resp.Results[0]
	return &CompletionResponse{
		Text:   result.OutputText,
		Tokens: result.TokenCount,
	}, nil
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
