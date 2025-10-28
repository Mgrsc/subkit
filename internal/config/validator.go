package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Type    string   // "rule-provider", "rule-set", "proxy-group"
	Issues  []string // List of specific issues found
	Missing []string // List of missing items
}

// ValidationResult holds all validation errors
type ValidationResult struct {
	Errors          []ValidationError
	HasErrors       bool
	ErrorMessage    string // Formatted error message for LLM
	AvailableGeoIP  []string
	AvailableGeoSite []string
}

// ConfigValidator validates the generated configuration
type ConfigValidator struct {
	geoipFiles   []string
	geositeFiles []string
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator(geoipFiles, geositeFiles []string) *ConfigValidator {
	return &ConfigValidator{
		geoipFiles:   geoipFiles,
		geositeFiles: geositeFiles,
	}
}

// Validate performs comprehensive validation on the configuration
func (v *ConfigValidator) Validate(configData []byte) (*ValidationResult, error) {
	result := &ValidationResult{
		Errors:           []ValidationError{},
		HasErrors:        false,
		AvailableGeoIP:   v.geoipFiles,
		AvailableGeoSite: v.geositeFiles,
	}

	// Parse configuration
	var config map[string]interface{}
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Validate rule-providers
	providerIssues := v.validateRuleProviders(config)
	if len(providerIssues.Issues) > 0 || len(providerIssues.Missing) > 0 {
		result.Errors = append(result.Errors, providerIssues)
		result.HasErrors = true
	}

	// Extract valid provider names and proxy group names
	validProviders := v.extractProviderNames(config)
	validProxyGroups := v.extractProxyGroupNames(config)

	// Validate rules
	ruleSetIssues := v.validateRuleSetReferences(config, validProviders)
	if len(ruleSetIssues.Issues) > 0 || len(ruleSetIssues.Missing) > 0 {
		result.Errors = append(result.Errors, ruleSetIssues)
		result.HasErrors = true
	}

	proxyGroupIssues := v.validateProxyGroupReferences(config, validProxyGroups)
	if len(proxyGroupIssues.Issues) > 0 || len(proxyGroupIssues.Missing) > 0 {
		result.Errors = append(result.Errors, proxyGroupIssues)
		result.HasErrors = true
	}

	// Build error message for LLM if there are errors
	if result.HasErrors {
		result.ErrorMessage = v.buildErrorMessage(result)
	}

	return result, nil
}

// validateRuleProviders checks if rule-provider URLs reference existing files
func (v *ConfigValidator) validateRuleProviders(config map[string]interface{}) ValidationError {
	validationErr := ValidationError{
		Type:    "rule-provider",
		Issues:  []string{},
		Missing: []string{},
	}

	ruleProviders, ok := config["rule-providers"].(map[string]interface{})
	if !ok {
		return validationErr
	}

	for name, providerData := range ruleProviders {
		provider, ok := providerData.(map[string]interface{})
		if !ok {
			continue
		}

		// Check URL field
		url, ok := provider["url"].(string)
		if !ok {
			continue
		}

		// Extract file path from URL
		// Expected format: https://raw.githubusercontent.com/.../geosite/apple.yaml
		if strings.Contains(url, "/geosite/") {
			fileName := extractFileName(url)
			if !v.fileExists(fileName, v.geositeFiles) {
				validationErr.Issues = append(validationErr.Issues,
					fmt.Sprintf("Rule provider '%s' references non-existent GeoSite file: %s", name, fileName))
				validationErr.Missing = append(validationErr.Missing, fileName)
			}
		} else if strings.Contains(url, "/geoip/") {
			fileName := extractFileName(url)
			if !v.fileExists(fileName, v.geoipFiles) {
				validationErr.Issues = append(validationErr.Issues,
					fmt.Sprintf("Rule provider '%s' references non-existent GeoIP file: %s", name, fileName))
				validationErr.Missing = append(validationErr.Missing, fileName)
			}
		}
	}

	return validationErr
}

// validateRuleSetReferences checks if rules reference defined rule-providers
func (v *ConfigValidator) validateRuleSetReferences(config map[string]interface{}, validProviders map[string]bool) ValidationError {
	validationErr := ValidationError{
		Type:    "rule-set",
		Issues:  []string{},
		Missing: []string{},
	}

	rules, ok := config["rules"].([]interface{})
	if !ok {
		return validationErr
	}

	for _, rule := range rules {
		ruleStr, ok := rule.(string)
		if !ok {
			continue
		}

		// Check if it's a RULE-SET rule
		if strings.HasPrefix(ruleStr, "RULE-SET,") {
			parts := strings.Split(ruleStr, ",")
			if len(parts) < 2 {
				continue
			}

			ruleSetName := strings.TrimSpace(parts[1])
			if !validProviders[ruleSetName] {
				validationErr.Issues = append(validationErr.Issues,
					fmt.Sprintf("Rule references undefined rule-set: %s", ruleSetName))
				validationErr.Missing = append(validationErr.Missing, ruleSetName)
			}
		}
	}

	return validationErr
}

// validateProxyGroupReferences checks if rules reference defined proxy-groups
func (v *ConfigValidator) validateProxyGroupReferences(config map[string]interface{}, validProxyGroups map[string]bool) ValidationError {
	validationErr := ValidationError{
		Type:    "proxy-group",
		Issues:  []string{},
		Missing: []string{},
	}

	rules, ok := config["rules"].([]interface{})
	if !ok {
		return validationErr
	}

	// Built-in targets that don't need to be in proxy-groups
	builtinTargets := map[string]bool{
		"DIRECT":          true,
		"REJECT":          true,
		"REJECT-DROP":     true,
		"PASS":            true,
		"COMPATIBLE":      true,
	}

	for _, rule := range rules {
		ruleStr, ok := rule.(string)
		if !ok {
			continue
		}

		// Parse rule format: TYPE,param1,param2,...,PROXY-GROUP[,no-resolve]
		parts := strings.Split(ruleStr, ",")
		if len(parts) < 2 {
			continue
		}

		// The proxy group is typically in the 3rd position for RULE-SET, DOMAIN, etc.
		// For MATCH, it's in the 2nd position
		ruleType := strings.TrimSpace(parts[0])
		var proxyGroup string

		if ruleType == "MATCH" {
			if len(parts) >= 2 {
				proxyGroup = strings.TrimSpace(parts[1])
			}
		} else if ruleType == "GEOIP" {
			// GEOIP,CN,PROXY-GROUP[,no-resolve]
			if len(parts) >= 3 {
				proxyGroup = strings.TrimSpace(parts[2])
			}
		} else {
			// RULE-SET, DOMAIN, etc.: TYPE,param,PROXY-GROUP[,no-resolve]
			if len(parts) >= 3 {
				proxyGroup = strings.TrimSpace(parts[2])
			}
		}

		// Check if proxy group exists (skip built-in targets and empty)
		if proxyGroup != "" && !builtinTargets[proxyGroup] && !validProxyGroups[proxyGroup] {
			validationErr.Issues = append(validationErr.Issues,
				fmt.Sprintf("Rule references undefined proxy-group: %s (in rule: %s)", proxyGroup, ruleStr))
			if !contains(validationErr.Missing, proxyGroup) {
				validationErr.Missing = append(validationErr.Missing, proxyGroup)
			}
		}
	}

	return validationErr
}

// extractProviderNames extracts all rule-provider names from config
func (v *ConfigValidator) extractProviderNames(config map[string]interface{}) map[string]bool {
	providers := make(map[string]bool)

	ruleProviders, ok := config["rule-providers"].(map[string]interface{})
	if !ok {
		return providers
	}

	for name := range ruleProviders {
		providers[name] = true
	}

	return providers
}

// extractProxyGroupNames extracts all proxy-group names from config
func (v *ConfigValidator) extractProxyGroupNames(config map[string]interface{}) map[string]bool {
	groups := make(map[string]bool)

	proxyGroups, ok := config["proxy-groups"].([]interface{})
	if !ok {
		return groups
	}

	for _, groupData := range proxyGroups {
		group, ok := groupData.(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := group["name"].(string)
		if ok {
			groups[name] = true
		}
	}

	return groups
}

// fileExists checks if a file exists in the available files list
func (v *ConfigValidator) fileExists(fileName string, availableFiles []string) bool {
	for _, file := range availableFiles {
		if file == fileName {
			return true
		}
	}
	return false
}

// extractFileName extracts the file name from a URL
func extractFileName(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

// buildErrorMessage builds a formatted error message for LLM correction
func (v *ConfigValidator) buildErrorMessage(result *ValidationResult) string {
	var builder strings.Builder

	builder.WriteString("⚠️ Configuration validation found the following issues:\n\n")

	for _, err := range result.Errors {
		switch err.Type {
		case "rule-provider":
			builder.WriteString("**Rule Provider Issues:**\n")
			for _, issue := range err.Issues {
				builder.WriteString(fmt.Sprintf("  - %s\n", issue))
			}
			builder.WriteString("\n")

		case "rule-set":
			builder.WriteString("**Rule Set Reference Issues:**\n")
			for _, issue := range err.Issues {
				builder.WriteString(fmt.Sprintf("  - %s\n", issue))
			}
			if len(err.Missing) > 0 {
				builder.WriteString(fmt.Sprintf("  Missing rule-sets: %s\n", strings.Join(err.Missing, ", ")))
			}
			builder.WriteString("\n")

		case "proxy-group":
			builder.WriteString("**Proxy Group Reference Issues:**\n")
			for _, issue := range err.Issues {
				builder.WriteString(fmt.Sprintf("  - %s\n", issue))
			}
			if len(err.Missing) > 0 {
				builder.WriteString(fmt.Sprintf("  Missing proxy-groups: %s\n", strings.Join(err.Missing, ", ")))
			}
			builder.WriteString("\n")
		}
	}

	builder.WriteString("Please correct these issues.\n\n")
	builder.WriteString("<available_GeoIP>\n")
	builder.WriteString(strings.Join(result.AvailableGeoIP, "\n"))
	builder.WriteString("\n</available_GeoIP>\n\n")
	builder.WriteString("<Available_GeoSite>\n")
	builder.WriteString(strings.Join(result.AvailableGeoSite, "\n"))
	builder.WriteString("\n</Available_GeoSite>\n")

	return builder.String()
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
