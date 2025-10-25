package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"subkit/internal/logger"
)

type Client struct {
	baseURL                  string
	apiKey                   string
	client                   *http.Client
	model                    string
	proxyGroupsSystemPrompt  string
	proxyGroupsUserPrompt    string
	rulesSystemPrompt        string
	rulesUserPrompt          string
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

func NewClient() (*Client, error) {
	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("LLM_API_KEY not set")
	}

	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "gpt-5-mini"
	}

	timeoutStr := os.Getenv("LLM_TIMEOUT")
	timeout := 120 * time.Second
	if timeoutStr != "" {
		if seconds, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = seconds
			logger.Info("[LLM] Using custom timeout: %v", timeout)
		} else {
			logger.Warn("[LLM] Invalid LLM_TIMEOUT format, using default: 120s")
		}
	}

	proxyGroupsSystemPrompt, err := loadPrompt("config/prompts/proxy_groups_system.txt")
	if err != nil {
		logger.Warn("[LLM] Failed to load proxy_groups_system prompt, using default: %v", err)
		proxyGroupsSystemPrompt = getDefaultProxyGroupsSystemPrompt()
	} else {
		logger.Info("[LLM] Loaded custom proxy_groups_system prompt")
	}

	proxyGroupsUserPrompt, err := loadPrompt("config/prompts/proxy_groups_user.txt")
	if err != nil {
		logger.Warn("[LLM] Failed to load proxy_groups_user prompt, using default: %v", err)
		proxyGroupsUserPrompt = getDefaultProxyGroupsUserPrompt()
	} else {
		logger.Info("[LLM] Loaded custom proxy_groups_user prompt")
	}

	rulesSystemPrompt, err := loadPrompt("config/prompts/rules_system.txt")
	if err != nil {
		logger.Warn("[LLM] Failed to load rules_system prompt, using default: %v", err)
		rulesSystemPrompt = getDefaultRulesSystemPrompt()
	} else {
		logger.Info("[LLM] Loaded custom rules_system prompt")
	}

	rulesUserPrompt, err := loadPrompt("config/prompts/rules_user.txt")
	if err != nil {
		logger.Warn("[LLM] Failed to load rules_user prompt, using default: %v", err)
		rulesUserPrompt = getDefaultRulesUserPrompt()
	} else {
		logger.Info("[LLM] Loaded custom rules_user prompt")
	}

	return &Client{
		baseURL:                 baseURL,
		apiKey:                  apiKey,
		model:                   model,
		proxyGroupsSystemPrompt: proxyGroupsSystemPrompt,
		proxyGroupsUserPrompt:   proxyGroupsUserPrompt,
		rulesSystemPrompt:       rulesSystemPrompt,
		rulesUserPrompt:         rulesUserPrompt,
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func loadPrompt(filepath string) (string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func getDefaultProxyGroupsSystemPrompt() string {
	return `You are a Mihomo/Clash proxy configuration expert.
Generate high-performance, maintainable proxy-groups configuration based on available proxies and user requirements.

Output must start with "proxy-groups:" and be valid YAML only. Use 2-space indentation.`
}

func getDefaultProxyGroupsUserPrompt() string {
	return `<Available proxies>
{PROXIES}
</Available proxies>

<CUSTOM_REQUIREMENTS>
{CUSTOM_REQUIREMENTS}
</CUSTOM_REQUIREMENTS>`
}

func getDefaultRulesSystemPrompt() string {
	return `You are a network routing expert specializing in Mihomo/Clash configurations.
Generate rules and rule-providers configuration in YAML format.

Output must include both "rule-providers:" and "rules:" sections in valid YAML only.`
}

func getDefaultRulesUserPrompt() string {
	return `<proxy_group_configuration>
{PROXY_GROUPS}
</proxy_group_configuration>

<available_GeoIP>
{GEOIP_FILES}
</available_GeoIP>

<Available_GeoSite>
{GEOSITE_FILES}
</Available_GeoSite>

<CUSTOM_REQUIREMENTS>
{CUSTOM_REQUIREMENTS}
</CUSTOM_REQUIREMENTS>`
}

func (c *Client) Chat(messages []ChatMessage) (string, error) {
	logger.Info("[LLM] Sending request to %s with model %s", c.baseURL, c.model)
	reqBody := ChatRequest{
		Model:    c.model,
		Messages: messages,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %w", err)
	}

	if logger.IsDebug() {
		logger.Debug("[LLM] Request payload:")
		logger.Debug("[LLM] Model: %s", c.model)
		logger.Debug("[LLM] Messages count: %d", len(messages))
		for i, msg := range messages {
			logger.Debug("[LLM] Message[%d] Role: %s", i, msg.Role)
			logger.Debug("[LLM] Message[%d] Content length: %d chars", i, len(msg.Content))
			logger.Debug("[LLM] Message[%d] Content:\n%s", i, msg.Content)
		}
	}

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	logger.Info("[LLM] Waiting for LLM response...")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Error("[LLM] Request failed with status %d: %s", resp.StatusCode, string(body))
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("decode response failed: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices")
	}

	responseContent := chatResp.Choices[0].Message.Content
	logger.Info("[LLM] Received response, length: %d chars", len(responseContent))

	if logger.IsDebug() {
		logger.Debug("[LLM] Response content:\n%s", responseContent)
	}

	return responseContent, nil
}

func (c *Client) GenerateProxyGroups(proxies, customRequirements string) (string, error) {
	logger.Info("[LLM] Generating proxy-groups configuration...")

	userPrompt := strings.ReplaceAll(c.proxyGroupsUserPrompt, "{PROXIES}", proxies)

	if customRequirements != "" {
		userPrompt = strings.ReplaceAll(userPrompt, "{CUSTOM_REQUIREMENTS}", customRequirements)
		logger.Info("[LLM] Including custom requirements in proxy-groups generation")
	} else {
		userPrompt = strings.ReplaceAll(userPrompt, "{CUSTOM_REQUIREMENTS}", "No special requirements")
	}

	messages := []ChatMessage{
		{Role: "system", Content: c.proxyGroupsSystemPrompt},
		{Role: "user", Content: userPrompt},
	}

	result, err := c.Chat(messages)
	if err == nil {
		logger.Info("[LLM] Proxy-groups generated successfully")
	}
	return result, err
}

func (c *Client) GenerateRules(proxyGroups, geoipBaseURL string, geoipFiles []string, geositeBaseURL string, geositeFiles []string, customRequirements string) (string, error) {
	logger.Info("[LLM] Generating rules and rule-providers configuration...")

	geoipList := formatFileList(geoipFiles)
	geositeList := formatFileList(geositeFiles)

	userPrompt := strings.ReplaceAll(c.rulesUserPrompt, "{PROXY_GROUPS}", proxyGroups)
	userPrompt = strings.ReplaceAll(userPrompt, "{GEOIP_FILES}", geoipList)
	userPrompt = strings.ReplaceAll(userPrompt, "{GEOSITE_FILES}", geositeList)

	if customRequirements != "" {
		userPrompt = strings.ReplaceAll(userPrompt, "{CUSTOM_REQUIREMENTS}", customRequirements)
		logger.Info("[LLM] Including custom requirements in rules generation")
	} else {
		userPrompt = strings.ReplaceAll(userPrompt, "{CUSTOM_REQUIREMENTS}", "No special requirements")
	}

	messages := []ChatMessage{
		{Role: "system", Content: c.rulesSystemPrompt},
		{Role: "user", Content: userPrompt},
	}

	result, err := c.Chat(messages)
	if err == nil {
		logger.Info("[LLM] Rules and rule-providers generated successfully")
	}
	return result, err
}

func formatFileList(files []string) string {
	if len(files) == 0 {
		return "none"
	}

	return strings.Join(files, "\n")
}
