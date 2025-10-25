package converter

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Extractor struct {
	client *http.Client
}

func NewExtractor() *Extractor {
	return &Extractor{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (e *Extractor) ExtractFromURL(subURL string) ([]*ProxyNode, error) {
	log.Printf("[Extractor] Fetching subscription from URL: %s", subURL)
	resp, err := e.client.Get(subURL)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Printf("[Extractor] Successfully fetched subscription, reading content...")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	log.Printf("[Extractor] Content size: %d bytes", len(body))
	return e.ExtractFromContent(string(body))
}

func (e *Extractor) ExtractFromContent(content string) ([]*ProxyNode, error) {
	content = strings.TrimSpace(content)

	if isYAML(content) {
		log.Printf("[Extractor] Detected YAML format, parsing...")
		return e.extractFromYAML(content)
	}

	log.Printf("[Extractor] Attempting base64 decode...")
	return e.extractFromBase64(content)
}

func (e *Extractor) extractFromYAML(content string) ([]*ProxyNode, error) {
	var config MihomoConfig
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, fmt.Errorf("parse yaml failed: %w", err)
	}

	if len(config.Proxies) == 0 {
		return nil, fmt.Errorf("no proxies found in yaml")
	}

	log.Printf("[Extractor] Successfully parsed %d nodes from YAML", len(config.Proxies))
	nodes := make([]*ProxyNode, len(config.Proxies))
	for i := range config.Proxies {
		nodes[i] = &config.Proxies[i]
	}

	return nodes, nil
}

func (e *Extractor) extractFromBase64(content string) ([]*ProxyNode, error) {
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(content)
		if err != nil {
			return nil, fmt.Errorf("base64 decode failed: %w", err)
		}
	}

	lines := strings.Split(string(decoded), "\n")
	var nodes []*ProxyNode

	log.Printf("[Extractor] Processing %d lines from base64 content", len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		node, err := UriToProxy(line)
		if err != nil {
			log.Printf("[Extractor] Line %d parse failed: %v", i+1, err)
			continue
		}

		nodes = append(nodes, node)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no valid nodes found")
	}

	log.Printf("[Extractor] Successfully parsed %d valid nodes", len(nodes))
	return nodes, nil
}

func isYAML(content string) bool {
	content = strings.TrimSpace(content)
	return strings.HasPrefix(content, "proxies:") ||
		strings.HasPrefix(content, "proxy-groups:") ||
		strings.Contains(content, "\nproxies:") ||
		strings.Contains(content, "\nproxy-groups:")
}

func (e *Extractor) ExtractFromURIs(uris []string) ([]*ProxyNode, error) {
	var nodes []*ProxyNode

	for _, uri := range uris {
		node, err := UriToProxy(uri)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no valid nodes found")
	}

	return nodes, nil
}
