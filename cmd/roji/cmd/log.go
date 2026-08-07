package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kan/roji/config"
	"github.com/kan/roji/i18n"
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

	// A negative count would make the tail start past the end of the file,
	// which panics on the slice rather than showing anything.
	if logLines < 0 {
		return fmt.Errorf("--lines must not be negative, got %d (0 shows every line)", logLines)
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
	start := max(len(lines)-n, 0)
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

	// Print the last n lines, or every line for 0 — the same meaning --lines
	// carries in printLogTail and in the flag's own help.
	start := 0
	if initialLines > 0 {
		start = max(len(lines)-initialLines, 0)
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}

	// Follow from the end, so only new output is printed.
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	fmt.Printf("\n%s\n\n", i18n.Tf("cmd.log.following", logPath))

	// Follow new lines
	buf := make([]byte, 4096)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read: %w", err)
		}

		if n > 0 {
			os.Stdout.Write(buf[:n])
		} else {
			// The server rotates by renaming (see rotateLogFile), so the handle
			// held here follows the renamed file and simply stops producing
			// output. Watching its size cannot detect that: nothing truncates
			// it, and Stat on the handle reports the renamed inode. Compare the
			// path against what is held instead, and reopen when they differ.
			reopened, err := reopenIfRotated(file, logPath)
			if err != nil {
				return err
			}
			if reopened != nil {
				file.Close()
				file = reopened
				fmt.Printf("\n%s\n", i18n.T("cmd.log.rotated"))
			}

			// Sleep briefly before checking again
			// Using a simple sleep instead of fsnotify to keep dependencies minimal
			sleepMs(100)
		}
	}
}

// reopenIfRotated returns a handle on logPath when it is no longer the file
// that open refers to, or nil when nothing has changed.
//
// A missing path is not an error: the server recreates the log on its next
// write, so the caller keeps following the old handle until it appears.
func reopenIfRotated(open *os.File, logPath string) (*os.File, error) {
	held, err := open.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat log file: %w", err)
	}

	current, err := os.Stat(logPath)
	if err != nil || os.SameFile(held, current) {
		return nil, nil
	}

	reopened, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to reopen log file after rotation: %w", err)
	}
	return reopened, nil
}

// sleepMs sleeps for the specified number of milliseconds
func sleepMs(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
