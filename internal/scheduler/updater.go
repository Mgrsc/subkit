package scheduler

import (
	"encoding/json"
	"fmt"
	"io"
	"subkit/internal/logger"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	githubAPIBase  = "https://api.github.com/repos/MetaCubeX/meta-rules-dat/contents/geo"
	githubTreeAPI  = "https://api.github.com/repos/MetaCubeX/meta-rules-dat/git/trees"
	rawContentBase = "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo"
)

type Updater struct {
	interval   time.Duration
	client     *http.Client
	onComplete func()
}

func NewUpdater(intervalDays int) *Updater {
	return &Updater{
		interval: time.Duration(intervalDays) * 24 * time.Hour,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (u *Updater) SetOnComplete(callback func()) {
	u.onComplete = callback
}

func (u *Updater) Start() {
	u.updateRuleLists()

	ticker := time.NewTicker(u.interval)
	go func() {
		for range ticker.C {
			u.updateRuleLists()
		}
	}()
}

func (u *Updater) updateRuleLists() {
	logger.Info("Starting rule lists update...")

	if err := u.updateGeoIPList(); err != nil {
		logger.Error("Update geoip list failed: %v", err)
	} else {
		logger.Info("GeoIP list updated successfully")
	}

	if err := u.updateGeoSiteList(); err != nil {
		logger.Error("Update geosite list failed: %v", err)
	} else {
		logger.Info("GeoSite list updated successfully")
	}


	if u.onComplete != nil {
		logger.Info("[Updater] Notifying update completion...")
		u.onComplete()
	}
}

func (u *Updater) UpdateGeoIPManual() error {
	return u.updateGeoIPList()
}

func (u *Updater) UpdateGeoSiteManual() error {
	return u.updateGeoSiteList()
}

func (u *Updater) updateGeoIPList() error {
	return u.updateRuleList("geoip", "config/rules/geoip_files_yaml.txt")
}

func (u *Updater) updateGeoSiteList() error {
	return u.updateRuleList("geosite", "config/rules/geosite_files_yaml.txt")
}

func (u *Updater) updateRuleList(subdir, outputPath string) error {
	logger.Info("[Updater] Fetching %s list using Tree API...", subdir)

	sha, err := u.getTreeSHA(subdir)
	if err != nil {
		return fmt.Errorf("get tree SHA failed: %w", err)
	}
	logger.Info("[Updater] Got %s tree SHA: %s", subdir, sha)

	files, err := u.getFilesFromTree(sha)
	if err != nil {
		return fmt.Errorf("get files from tree failed: %w", err)
	}
	logger.Info("[Updater] Fetched %d files from %s tree", len(files), subdir)

	yamlFiles := u.filterYamlFiles(files)
	logger.Info("[Updater] Filtered to %d .yaml files (excluded 'classical' directory)", len(yamlFiles))

	if err := u.saveFilteredFiles(outputPath, yamlFiles); err != nil {
		return fmt.Errorf("save files failed: %w", err)
	}

	logger.Info("[Updater] Saved %d files to %s", len(yamlFiles), outputPath)
	return nil
}

func (u *Updater) getTreeSHA(subdir string) (string, error) {
	url := fmt.Sprintf("%s?ref=meta", githubAPIBase)
	resp, err := u.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %w", err)
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(body, &items); err != nil {
		return "", fmt.Errorf("parse json failed: %w", err)
	}

	for _, item := range items {
		if name, ok := item["name"].(string); ok && name == subdir {
			if itemType, ok := item["type"].(string); ok && itemType == "dir" {
				if sha, ok := item["sha"].(string); ok {
					return sha, nil
				}
			}
		}
	}

	return "", fmt.Errorf("subdirectory '%s' not found", subdir)
}

func (u *Updater) getFilesFromTree(sha string) ([]string, error) {
	url := fmt.Sprintf("%s/%s?recursive=1", githubTreeAPI, sha)
	resp, err := u.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch tree failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var treeData map[string]interface{}
	if err := json.Unmarshal(body, &treeData); err != nil {
		return nil, fmt.Errorf("parse json failed: %w", err)
	}

	if truncated, ok := treeData["truncated"].(bool); ok && truncated {
		return nil, fmt.Errorf("tree is truncated (too large)")
	}

	var files []string
	if tree, ok := treeData["tree"].([]interface{}); ok {
		for _, item := range tree {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemType, ok := itemMap["type"].(string); ok && itemType == "blob" {
					if path, ok := itemMap["path"].(string); ok {
						files = append(files, path)
					}
				}
			}
		}
	}

	return files, nil
}

func (u *Updater) filterYamlFiles(files []string) []string {
	var yamlFiles []string
	excludeDirs := []string{"classical"}

	for _, filePath := range files {
		shouldExclude := false
		for _, excludeDir := range excludeDirs {
			if strings.HasPrefix(filePath, excludeDir+"/") || strings.Contains(filePath, "/"+excludeDir+"/") {
				shouldExclude = true
				break
			}
		}

		if shouldExclude {
			continue
		}
		if strings.HasSuffix(filePath, ".yaml") {
			yamlFiles = append(yamlFiles, filePath)
		}
	}

	return yamlFiles
}

func (u *Updater) saveFilteredFiles(filepath string, files []string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("create file failed: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(fmt.Sprintf("%d\n", len(files))); err != nil {
		return fmt.Errorf("write count failed: %w", err)
	}

	for _, filename := range files {
		if _, err := file.WriteString(filename + "\n"); err != nil {
			return fmt.Errorf("write filename failed: %w", err)
		}
	}

	return nil
}
