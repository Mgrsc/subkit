package main

import (
	"fmt"
	"subkit/internal/logger"
	"os"

	"subkit/internal/scheduler"
)

func main() {
	logger.Init(); //(log.LstdFlags | log.Lshortfile)

	if _, err := os.Stat("config"); os.IsNotExist(err) {
		logger.Error("ERROR: This tool must be run from the project root directory")
		logger.Error("Usage: go run cmd/update-rules/main.go")
		os.Exit(1)
	}

	logger.Info("========================================")
	logger.Info("   Subkit - Manual Rules Updater")
	logger.Info("========================================")

	updater := scheduler.NewUpdater(0)

	logger.Info("Starting manual rule lists update...")
	logger.Info("This will download the latest GeoIP and GeoSite rules from GitHub...")
	logger.Info("")

	if err := os.MkdirAll("config/rules", 0755); err != nil {
		logger.Error("Failed to create config/rules directory: %v", err)
	}

	logger.Info("[1/2] Updating GeoIP list...")
	if err := updater.UpdateGeoIPManual(); err != nil {
		logger.Info("❌ Failed to update GeoIP list: %v", err)
	} else {
		logger.Info("✅ GeoIP list updated successfully")
	}

	logger.Info("[2/2] Updating GeoSite list...")
	if err := updater.UpdateGeoSiteManual(); err != nil {
		logger.Info("❌ Failed to update GeoSite list: %v", err)
	} else {
		logger.Info("✅ GeoSite list updated successfully")
	}

	logger.Info("")
	logger.Info("========================================")
	logger.Info("Update complete!")
	logger.Info("========================================")
	fmt.Println("\nRule lists have been updated to the latest version.")
	fmt.Println("Restart the server to use the new rules.")
}
