package scheduler

import (
	"encoding/json"
	"fmt"
	"io"
	"subkit/internal/logger"
	"net/http"
	"os"
	"time"
)

const (
	githubAPIBase  = "https://api.github.com/repos/MetaCubeX/meta-rules-dat/contents/geo"
	rawContentBase = "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo"
)

type Updater struct {
	interval time.Duration
	client   *http.Client
}

func NewUpdater(intervalDays int) *Updater {
	return &Updater{
		interval: time.Duration(intervalDays) * 24 * time.Hour,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
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
}

func (u *Updater) UpdateGeoIPManual() error {
	return u.updateGeoIPList()
}

func (u *Updater) UpdateGeoSiteManual() error {
	return u.updateGeoSiteList()
}

func (u *Updater) updateGeoIPList() error {
	url := fmt.Sprintf("%s/geoip?ref=meta", githubAPIBase)
	baseURL := fmt.Sprintf("BASE_URL: %s/geoip", rawContentBase)
	return u.fetchAndSaveWithBaseURL(url, "config/rules/geoip_list.txt", baseURL)
}

func (u *Updater) updateGeoSiteList() error {
	url := fmt.Sprintf("%s/geosite?ref=meta", githubAPIBase)
	baseURL := fmt.Sprintf("BASE_URL: %s/geosite", rawContentBase)
	return u.fetchAndSaveWithBaseURL(url, "config/rules/geosite_list.txt", baseURL)
}

func (u *Updater) fetchAndSaveWithBaseURL(url, filepath, baseURL string) error {
	resp, err := u.client.Get(url)
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(body, &items); err != nil {
		return fmt.Errorf("parse json failed: %w", err)
	}

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("create file failed: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(baseURL + "\n"); err != nil {
		return fmt.Errorf("write base url failed: %w", err)
	}

	for _, item := range items {
		if name, ok := item["name"].(string); ok {
			if _, err := file.WriteString(name + "\n"); err != nil {
				return fmt.Errorf("write filename failed: %w", err)
			}
		}
	}

	return nil
}
