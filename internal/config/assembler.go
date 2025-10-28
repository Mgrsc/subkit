package config

import (
	"fmt"
	"regexp"
	"strconv"
	"subkit/internal/logger"
	"os"
	"strings"
	"sync"

	"subkit/internal/converter"
	"subkit/internal/llm"

	"gopkg.in/yaml.v3"
)

type Assembler struct {
	llmClient       *llm.Client
	globalConfig    map[string]interface{}
	geoipBaseURL    string
	geoipFiles      []string
	geositeBaseURL  string
	geositeFiles    []string
	rulesMu         sync.RWMutex
	rulesLoaded     bool
}

func NewAssembler() (*Assembler, error) {
	llmClient, err := llm.NewClient()
	if err != nil {
		return nil, fmt.Errorf("create llm client failed: %w", err)
	}

	globalData, err := os.ReadFile("config/global.yaml")
	if err != nil {
		return nil, fmt.Errorf("read global config failed: %w", err)
	}

	var globalConfig map[string]interface{}
	if err := yaml.Unmarshal(globalData, &globalConfig); err != nil {
		return nil, fmt.Errorf("parse global config failed: %w", err)
	}

	return &Assembler{
		llmClient:    llmClient,
		globalConfig: globalConfig,
	}, nil
}

func (a *Assembler) LoadRuleLists() error {
	a.rulesMu.Lock()
	defer a.rulesMu.Unlock()

	geoipData, err := os.ReadFile("config/rules/geoip_files_yaml.txt")
	if err == nil {
		a.geoipBaseURL, a.geoipFiles = parseFilteredRuleList(string(geoipData), "geoip")
		logger.Info("[Assembler] Loaded %d GeoIP .yaml rules from %s", len(a.geoipFiles), a.geoipBaseURL)
	} else {
		logger.Warn("[Assembler] Failed to load geoip_files_yaml.txt: %v", err)
	}

	geositeData, err := os.ReadFile("config/rules/geosite_files_yaml.txt")
	if err == nil {
		a.geositeBaseURL, a.geositeFiles = parseFilteredRuleList(string(geositeData), "geosite")
		logger.Info("[Assembler] Loaded %d GeoSite .yaml rules from %s", len(a.geositeFiles), a.geositeBaseURL)
	} else {
		logger.Warn("[Assembler] Failed to load geosite_files_yaml.txt: %v", err)
	}

	a.rulesLoaded = true
	return nil
}

func (a *Assembler) ensureRulesLoaded() {
	a.rulesMu.RLock()
	loaded := a.rulesLoaded
	a.rulesMu.RUnlock()

	if !loaded {
		logger.Info("[Assembler] Rules not loaded yet, loading now...")
		a.LoadRuleLists()
	}
}

func parseFilteredRuleList(content string, ruleType string) (baseURL string, files []string) {
	baseURL = fmt.Sprintf("https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/%s", ruleType)

	lines := strings.Split(content, "\n")
	// First line is the count, skip it
	for i, line := range lines {
		if i == 0 {
			// Skip the count line
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		files = append(files, line)
	}
	return
}

func (a *Assembler) Assemble(nodes []*converter.ProxyNode) ([]byte, error) {
	return a.AssembleWithRequirements(nodes, "")
}

// ProgressCallback is called when each step completes
type ProgressCallback func(step string)

func (a *Assembler) AssembleWithRequirements(nodes []*converter.ProxyNode, customRequirements string) ([]byte, error) {
	return a.AssembleWithProgress(nodes, customRequirements, nil)
}

func (a *Assembler) AssembleWithProgress(nodes []*converter.ProxyNode, customRequirements string, onProgress ProgressCallback) ([]byte, error) {
	logger.Info("[Assembler] Starting configuration assembly for %d nodes", len(nodes))

	logger.Info("[Assembler] Marshaling proxies to YAML...")
	proxiesYAML, err := marshalYAML(map[string]interface{}{"proxies": nodes})
	if err != nil {
		return nil, fmt.Errorf("marshal proxies failed: %w", err)
	}

	// Notify frontend BEFORE starting proxy-groups generation
	if onProgress != nil {
		onProgress("groups")
	}
	logger.Info("[Assembler] Generating proxy-groups with LLM...")
	proxyGroupsYAML, err := a.generateProxyGroups(string(proxiesYAML), customRequirements)
	if err != nil {
		return nil, fmt.Errorf("generate proxy groups failed: %w", err)
	}

	// Notify frontend BEFORE starting rules generation
	if onProgress != nil {
		onProgress("rules")
	}
	logger.Info("[Assembler] Generating rules and rule-providers with LLM...")
	rulesYAML, err := a.generateRules(proxyGroupsYAML, customRequirements)
	if err != nil {
		return nil, fmt.Errorf("generate rules failed: %w", err)
	}

	// Notify frontend BEFORE starting assembly
	if onProgress != nil {
		onProgress("assemble")
	}

	logger.Info("[Assembler] Merging configurations...")
	finalConfig, err := a.mergeConfiguration(nodes, proxyGroupsYAML, rulesYAML)
	if err != nil {
		return nil, fmt.Errorf("merge configuration failed: %w", err)
	}

	// Marshal initial configuration
	configData, err := marshalYAML(finalConfig)
	if err != nil {
		return nil, fmt.Errorf("marshal config failed: %w", err)
	}

	// Notify frontend BEFORE starting validation
	if onProgress != nil {
		onProgress("validate")
	}

	logger.Info("[Assembler] Validating configuration...")
	// Perform validation and correction loop
	configData, err = a.validateAndCorrect(configData, proxyGroupsYAML, customRequirements)
	if err != nil {
		return nil, fmt.Errorf("validation and correction failed: %w", err)
	}

	logger.Info("[Assembler] Configuration assembly completed successfully")
	return configData, nil
}

// mergeConfiguration merges all configuration sections
func (a *Assembler) mergeConfiguration(nodes []*converter.ProxyNode, proxyGroupsYAML, rulesYAML string) (map[string]interface{}, error) {
	finalConfig := make(map[string]interface{})

	for k, v := range a.globalConfig {
		finalConfig[k] = v
	}

	finalConfig["proxies"] = nodes

	pgData := extractYAMLSection(proxyGroupsYAML, "proxy-groups")
	var pgMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(pgData), &pgMap); err == nil {
		if groups, ok := pgMap["proxy-groups"]; ok {
			finalConfig["proxy-groups"] = groups
			logger.Info("[Assembler] Merged proxy-groups successfully")
		}
	} else {
		logger.Info("[Assembler] Warning: Failed to parse proxy-groups: %v", err)
	}

	rulesData := extractYAMLSection(rulesYAML, "rules")
	var rulesMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(rulesData), &rulesMap); err == nil {
		if rules, ok := rulesMap["rules"]; ok {
			finalConfig["rules"] = rules
			logger.Info("[Assembler] Merged rules successfully")
		}
	} else {
		logger.Info("[Assembler] Warning: Failed to parse rules: %v", err)
	}

	rpData := extractYAMLSection(rulesYAML, "rule-providers")
	var rpMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(rpData), &rpMap); err == nil {
		if providers, ok := rpMap["rule-providers"]; ok {
			finalConfig["rule-providers"] = providers
			logger.Info("[Assembler] Merged rule-providers successfully")
		}
	} else {
		logger.Info("[Assembler] Warning: Failed to parse rule-providers: %v", err)
	}

	return finalConfig, nil
}

// validateAndCorrect validates configuration and corrects issues using LLM
func (a *Assembler) validateAndCorrect(configData []byte, proxyGroupsYAML, customRequirements string) ([]byte, error) {
	const maxRetries = 3

	// Ensure rules are loaded
	a.ensureRulesLoaded()

	// Safely read rule lists
	a.rulesMu.RLock()
	geoipFiles := a.geoipFiles
	geositeFiles := a.geositeFiles
	a.rulesMu.RUnlock()

	// Create validator
	validator := NewConfigValidator(geoipFiles, geositeFiles)

	for retry := 0; retry < maxRetries; retry++ {
		// Validate configuration
		result, err := validator.Validate(configData)
		if err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}

		// If no errors, return the configuration
		if !result.HasErrors {
			logger.Info("[Assembler] Configuration validation passed successfully")
			return configData, nil
		}

		// Log validation errors
		logger.Info("[Assembler] Validation found %d error(s), attempting correction (retry %d/%d)", len(result.Errors), retry+1, maxRetries)
		logger.Info("[Assembler] Validation errors:\n%s", result.ErrorMessage)

		// Try to correct using LLM
		correctedYAML, err := a.correctConfiguration(string(configData), proxyGroupsYAML, result.ErrorMessage, customRequirements)
		if err != nil {
			logger.Info("[Assembler] Warning: LLM correction failed: %v", err)
			// If it's the last retry, return the original config with a warning
			if retry == maxRetries-1 {
				logger.Info("[Assembler] Max retries reached, returning configuration with validation warnings")
				return configData, nil
			}
			continue
		}

		// Update configData with corrected version
		configData = []byte(correctedYAML)
		logger.Info("[Assembler] Applied LLM corrections, re-validating...")
	}

	// If we exhausted retries, return the last version
	logger.Info("[Assembler] Validation completed with warnings after %d attempts", maxRetries)
	return configData, nil
}

// correctConfiguration uses LLM to fix configuration validation errors
func (a *Assembler) correctConfiguration(currentConfig, proxyGroupsYAML, validationError, customRequirements string) (string, error) {
	logger.Info("[Assembler] Requesting LLM to correct configuration errors...")

	systemPrompt := `You are an expert in Mihomo (Clash Meta) proxy configuration.
Your task is to fix configuration errors based on validation feedback.

Rules:
1. Fix ONLY the specific issues mentioned in the validation errors
2. Maintain all existing proxy-groups and their configurations
3. Use only files that exist in the available GeoIP/GeoSite lists
4. Ensure all rules reference valid rule-providers and proxy-groups
5. Return the COMPLETE corrected configuration in YAML format
6. Do not remove or modify working sections

Output the complete corrected YAML configuration, starting with the top-level keys.`

	userPrompt := fmt.Sprintf(`Current configuration has validation errors. Please fix them.

Proxy Groups (for reference):
%s

Validation Errors:
%s

Current Configuration:
%s

Custom Requirements: %s

Please output the COMPLETE corrected configuration in valid YAML format.`,
		proxyGroupsYAML,
		validationError,
		currentConfig,
		func() string {
			if customRequirements != "" {
				return customRequirements
			}
			return "None"
		}())

	messages := []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	correctedYAML, err := a.llmClient.Chat(messages)
	if err != nil {
		return "", fmt.Errorf("LLM chat failed: %w", err)
	}

	// Extract YAML content if wrapped in code blocks
	correctedYAML = extractYAMLContent(correctedYAML)

	logger.Info("[Assembler] LLM correction completed")
	return correctedYAML, nil
}

// extractYAMLContent extracts YAML from markdown code blocks if present
func extractYAMLContent(content string) string {
	// Remove markdown code blocks if present
	content = strings.TrimSpace(content)

	// Check for ```yaml or ``` code blocks
	if strings.HasPrefix(content, "```yaml") {
		content = strings.TrimPrefix(content, "```yaml")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSpace(content)
	}

	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	return content
}

func marshalYAML(v interface{}) ([]byte, error) {
	var buf strings.Builder
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)

	if err := encoder.Encode(v); err != nil {
		return nil, err
	}

	if err := encoder.Close(); err != nil {
		return nil, err
	}

	// Unescape Unicode sequences to preserve emojis
	result := unescapeUnicode(buf.String())
	return []byte(result), nil
}

// unescapeUnicode converts Unicode escape sequences like \U0001F1FA to actual emojis
func unescapeUnicode(s string) string {
	// Match \U followed by 8 hex digits
	re := regexp.MustCompile(`\\U([0-9A-Fa-f]{8})`)

	return re.ReplaceAllStringFunc(s, func(match string) string {
		// Extract hex digits
		hexStr := match[2:] // Remove \U prefix
		codePoint, err := strconv.ParseInt(hexStr, 16, 64)
		if err != nil {
			return match // Return original if parsing fails
		}
		return string(rune(codePoint))
	})
}

func (a *Assembler) generateProxyGroups(proxiesYAML, customRequirements string) (string, error) {
	return a.llmClient.GenerateProxyGroups(proxiesYAML, customRequirements)
}

func (a *Assembler) generateRules(proxyGroupsYAML, customRequirements string) (string, error) {
	// Ensure rules are loaded before generating
	a.ensureRulesLoaded()

	// Safely read rule lists
	a.rulesMu.RLock()
	geoipBaseURL := a.geoipBaseURL
	geoipFiles := a.geoipFiles
	geositeBaseURL := a.geositeBaseURL
	geositeFiles := a.geositeFiles
	a.rulesMu.RUnlock()

	return a.llmClient.GenerateRules(
		proxyGroupsYAML,
		geoipBaseURL,
		geoipFiles,
		geositeBaseURL,
		geositeFiles,
		customRequirements,
	)
}

func extractYAMLSection(content, section string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inSection := false
	baseIndent := -1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, section+":") {
			inSection = true
			result = append(result, line)
			continue
		}

		if inSection {
			if trimmed == "" || strings.HasPrefix(line, "#") {
				result = append(result, line)
				continue
			}

			indent := len(line) - len(strings.TrimLeft(line, " "))

			if baseIndent == -1 && trimmed != "" {
				baseIndent = indent
			}

			if indent > 0 && (baseIndent == -1 || indent >= baseIndent) {
				result = append(result, line)
			} else if strings.Contains(line, ":") && !strings.HasPrefix(trimmed, "-") {
				break
			}
		}
	}

	return strings.Join(result, "\n")
}
