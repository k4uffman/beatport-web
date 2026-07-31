package main

import (
	"archive/zip"
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

	cmd := exec.Command("./beatportdl-cli", url)
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
		if err != nil || info.IsDir() {
			return err
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
