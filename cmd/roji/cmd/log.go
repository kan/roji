package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kan/roji/config"
	"github.com/spf13/cobra"
)

var (
	logLines    int
	logNoFollow bool
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "View roji server logs",
	Long: `View roji server logs from the log file.

By default, follows the log file in real-time (like tail -f).
Use --no-follow to print current logs and exit.`,
	RunE: runLog,
}

func init() {
	rootCmd.AddCommand(logCmd)

	logCmd.Flags().IntVarP(&logLines, "lines", "n", 20, "Number of lines to show (0 for all)")
	logCmd.Flags().BoolVar(&logNoFollow, "no-follow", false, "Print logs and exit (don't follow)")
}

func runLog(cmd *cobra.Command, args []string) error {
	logPath := config.LogFilePath()

	// Check if log file exists
	if !config.Exists(logPath) {
		return fmt.Errorf("log file not found: %s\nIs roji server running?", logPath)
	}

	if logNoFollow {
		return printLogTail(logPath, logLines)
	}

	return followLog(logPath, logLines)
}

// printLogTail prints the last n lines of the log file
func printLogTail(logPath string, n int) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	if n == 0 {
		// Print entire file
		_, err = io.Copy(os.Stdout, file)
		return err
	}

	// Read all lines and print last n
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	// Print last n lines
	start := len(lines) - n
	if start < 0 {
		start = 0
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}

	return nil
}

// followLog follows the log file in real-time
func followLog(logPath string, initialLines int) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// First, read and print last n lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	// Print last n lines
	start := len(lines) - initialLines
	if start < 0 {
		start = 0
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}

	// Get current position for following
	pos, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	fmt.Printf("\n--- Following %s (Ctrl+C to exit) ---\n\n", logPath)

	// Follow new lines
	buf := make([]byte, 4096)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read: %w", err)
		}

		if n > 0 {
			os.Stdout.Write(buf[:n])
			pos += int64(n)
		} else {
			// Check if file was truncated (rotated)
			info, err := file.Stat()
			if err == nil && info.Size() < pos {
				// File was truncated, seek to beginning
				file.Seek(0, io.SeekStart)
				pos = 0
				fmt.Println("\n--- Log file rotated ---")
			}

			// Sleep briefly before checking again
			// Using a simple sleep instead of fsnotify to keep dependencies minimal
			sleepMs(100)
		}
	}
}

// sleepMs sleeps for the specified number of milliseconds
func sleepMs(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
