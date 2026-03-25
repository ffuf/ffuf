package payloadtamper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// downloadTampers fetches tamper .go files from the ffuf GitHub repository
// and saves them to the specified directory.
func DownloadTampers(apiURL, destDir string, overwrite bool) error {
	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	// Fetch directory listing from GitHub API
	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("failed to fetch tamper list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var entries []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	var downloaded, skipped int
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name, ".go") {
			continue
		}

		destPath := filepath.Join(destDir, entry.Name)

		// Skip if file already exists
		if _, err := os.Stat(destPath); err == nil && !overwrite {
			skipped++
			continue
		}

		// Download the file
		fileResp, err := http.Get(entry.DownloadURL)
		if err != nil {
			return fmt.Errorf("failed to download %s: %w", entry.Name, err)
		}

		data, err := io.ReadAll(fileResp.Body)
		fileResp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", entry.Name, err)
		}

		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", destPath, err)
		}
		downloaded++
		fmt.Printf("\033[1;32m\u2714\033[0m %s\n", entry.Name)
	}

	fmt.Printf("\nTampers directory: %s\n", destDir)
	fmt.Printf("Downloaded: %d, Already existed: %d\n", downloaded, skipped)
	return nil
}
