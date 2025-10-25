package config

import (
	"fmt"
	"regexp"
	"strconv"
	"subkit/internal/logger"
	"os"
	"strings"

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
	geoipData, err := os.ReadFile("config/rules/geoip_list.txt")
	if err == nil {
		a.geoipBaseURL, a.geoipFiles = parseRuleList(string(geoipData))
		logger.Info("[Assembler] Loaded %d GeoIP rules from %s", len(a.geoipFiles), a.geoipBaseURL)
	}

	geositeData, err := os.ReadFile("config/rules/geosite_list.txt")
	if err == nil {
		a.geositeBaseURL, a.geositeFiles = parseRuleList(string(geositeData))
		logger.Info("[Assembler] Loaded %d GeoSite rules from %s", len(a.geositeFiles), a.geositeBaseURL)
	}

	return nil
}

func parseRuleList(content string) (baseURL string, files []string) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if i == 0 || strings.HasPrefix(line, "BASE_URL:") || strings.HasPrefix(line, "Base URL:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				baseURL = strings.TrimSpace(parts[1])
				continue
			}
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

	logger.Info("[Assembler] Merging configurations...")
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

	logger.Info("[Assembler] Configuration assembly completed successfully")
	if onProgress != nil {
		onProgress("assemble")
	}
	return marshalYAML(finalConfig)
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
	return a.llmClient.GenerateRules(
		proxyGroupsYAML,
		a.geoipBaseURL,
		a.geoipFiles,
		a.geositeBaseURL,
		a.geositeFiles,
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
