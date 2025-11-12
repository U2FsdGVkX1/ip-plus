package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/xiaoqidun/qqwry"
)

const (
	// IPDB file name
	ipdbFileName = "qqwry.ipdb"
	// CDN download URL
	ipdbDownloadURL = "https://cdn.jsdelivr.net/npm/qqwry.raw.ipdb/qqwry.ipdb"
)

// ensureIPDB checks if IP database exists, downloads if not
func ensureIPDB() error {
	// Get executable directory
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	ipdbPath := filepath.Join(exeDir, ipdbFileName)

	// Check if file exists
	if _, err := os.Stat(ipdbPath); err == nil {
		return nil // File already exists
	}

	// Download the database
	fmt.Fprintf(os.Stderr, "Downloading IP database...\n")

	resp, err := http.Get(ipdbDownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download IP database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download IP database: HTTP %d", resp.StatusCode)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp(exeDir, "qqwry-*.ipdb.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up on failure

	// Download with progress
	totalSize := resp.ContentLength
	downloaded := int64(0)
	buffer := make([]byte, 32*1024) // 32KB buffer

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := tmpFile.Write(buffer[:n]); writeErr != nil {
				tmpFile.Close()
				return fmt.Errorf("failed to write to temp file: %w", writeErr)
			}
			downloaded += int64(n)

			if totalSize > 0 {
				fmt.Fprintf(os.Stderr, "\rDownloading: %.2f MB / %.2f MB (%.1f%%)",
					float64(downloaded)/(1024*1024),
					float64(totalSize)/(1024*1024),
					float64(downloaded)*100/float64(totalSize))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to download: %w", err)
		}
	}

	tmpFile.Close()
	fmt.Fprintf(os.Stderr, "\nDownload complete!\n")

	// Rename temp file to final name
	if err := os.Rename(tmpPath, ipdbPath); err != nil {
		return fmt.Errorf("failed to move database file: %w", err)
	}

	return nil
}

// loadIPDB loads the IP database
func loadIPDB() error {
	// Get executable directory
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	ipdbPath := filepath.Join(exeDir, ipdbFileName)

	// Load the database
	if err := qqwry.LoadFile(ipdbPath); err != nil {
		return fmt.Errorf("failed to load IP database: %w", err)
	}

	return nil
}

func main() {
	// Check if command is provided
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [args...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s ss -nltp\n", os.Args[0])
		os.Exit(1)
	}

	// Ensure IP database exists
	if err := ensureIPDB(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please manually download database file to: %s\n", ipdbFileName)
		fmt.Fprintf(os.Stderr, "Download URL: %s\n", ipdbDownloadURL)
		os.Exit(1)
	}

	// Load IP database
	if err := loadIPDB(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Prepare command
	cmdName := os.Args[1]
	cmdArgs := []string{}
	if len(os.Args) > 2 {
		cmdArgs = os.Args[2:]
	}

	cmd := exec.Command(cmdName, cmdArgs...)

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating stdout pipe: %v\n", err)
		os.Exit(1)
	}

	// Pass through stderr directly
	cmd.Stderr = os.Stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting command: %v\n", err)
		os.Exit(1)
	}

	// Process output line by line
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		enrichedLine := EnrichLine(line)
		fmt.Println(enrichedLine)
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading command output: %v\n", err)
	}

	// Wait for command to finish
	if err := cmd.Wait(); err != nil {
		// Command failed, exit with its exit code
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		// Other error
		fmt.Fprintf(os.Stderr, "Error waiting for command: %v\n", err)
		os.Exit(1)
	}

	// Command succeeded
	os.Exit(0)
}
