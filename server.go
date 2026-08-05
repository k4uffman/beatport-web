package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	// Allow both GET and POST requests
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read from query string (GET) or form data (POST)
	url := r.URL.Query().Get("url")
	if url == "" {
		url = r.FormValue("url")
	}

	if url == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	workDir, err := os.MkdirTemp("", "bp-dl-*")
	if err != nil {
		http.Error(w, "Failed to create temp directory", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(workDir)

	// Pull secure credentials from Cloud Run
	username := os.Getenv("BEATPORT_USERNAME")
	password := os.Getenv("BEATPORT_PASSWORD")
	credentialsJSON := os.Getenv("BEATPORT_CREDENTIALS")

	// Generate the exact config format with the required downloads_directory
	configContent := fmt.Sprintf(`username: %s
password: %s
quality: lossless
key_system: camelot
track_exists: update
keep_cover: true
downloads_directory: downloads
`, username, password)

	err = os.WriteFile(filepath.Join(workDir, "beatportdl-config.yml"), []byte(configContent), 0600)
	if err != nil {
		http.Error(w, "Failed to write config file", http.StatusInternalServerError)
		return
	}

	if credentialsJSON != "" {
		err = os.WriteFile(filepath.Join(workDir, "beatportdl-credentials.json"), []byte(credentialsJSON), 0600)
		if err != nil {
			http.Error(w, "Failed to write credentials file", http.StatusInternalServerError)
			return
		}
	}

	cmd := exec.Command("/app/beatportdl-cli", url)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "HOME="+workDir, "XDG_CONFIG_HOME="+workDir)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("beatportdl execution failed: %v\nOutput: %s", err, string(output))
		http.Error(w, "Failed to download tracks from Beatport", http.StatusInternalServerError)
		return
	}

	// Tell the browser to expect a streamed ZIP download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"beatport_tracks.zip\"")

	// Create a zip writer that outputs directly to the HTTP stream
	archive := zip.NewWriter(w)

	// Collect all downloaded audio files
	var filesToZip []string
	filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() != "beatportdl-config.yml" && info.Name() != "beatportdl-credentials.json" {
			filesToZip = append(filesToZip, path)
		}
		return nil
	})

	// Stream each file to the client and delete it from server memory immediately
	for _, path := range filesToZip {
		relPath, _ := filepath.Rel(workDir, path)

		writer, err := archive.Create(relPath)
		if err != nil {
			continue
		}

		file, err := os.Open(path)
		if err != nil {
			continue
		}

		io.Copy(writer, file)
		file.Close()

		// Delete the raw file from RAM immediately after streaming
		os.Remove(path)
	}

	archive.Close()
}

func main() {
	http.HandleFunc("/api/download", enableCORS(handleDownload))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server listening on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
