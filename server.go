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
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	url := r.FormValue("url")
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

	// Generate the exact config format beatportdl expects
	configContent := fmt.Sprintf(`username: %s
password: %s
quality: lossless
key_system: camelot
track_exists: update
keep_cover: true
`, username, password)

	// Write the config file into the working directory
	err = os.WriteFile(filepath.Join(workDir, "beatportdl-config.yml"), []byte(configContent), 0600)
	if err != nil {
		http.Error(w, "Failed to write config file", http.StatusInternalServerError)
		return
	}

	// Run the beatportdl CLI
	cmd := exec.Command("/app/beatportdl-cli", url)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("beatportdl execution failed: %v\nOutput: %s", err, string(output))
		http.Error(w, "Failed to download tracks from Beatport", http.StatusInternalServerError)
		return
	}

	zipPath := filepath.Join(os.TempDir(), "beatport_download.zip")
	defer os.Remove(zipPath)

	if err := createZip(workDir, zipPath); err != nil {
		log.Printf("Zip creation failed: %v", err)
		http.Error(w, "Failed to package files into ZIP", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"beatport_tracks.zip\"")
	http.ServeFile(w, r, zipPath)
}

func createZip(sourceDir, targetZip string) error {
	zipFile, err := os.Create(targetZip)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		// Skip directories
		if err != nil || info.IsDir() {
			return nil
		}
		
		// CRITICAL: Do not zip the config or credentials files!
		if info.Name() == "beatportdl-config.yml" || info.Name() == "beatportdl-credentials.json" {
			return nil
		}
		
		relPath, _ := filepath.Rel(sourceDir, path)
		writer, err := archive.Create(relPath)
		if err != nil {
			return err
		}
		
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
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
