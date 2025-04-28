package rlog

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNewInvalidDirectory verifies that creating a Writer with a non-existent directory fails.
func TestNewInvalidDirectory(t *testing.T) {
	nonExistentDir := filepath.Join(os.TempDir(), "nonexistent-dir-for-testing")
	_, err := New(nonExistentDir)
	if err == nil {
		t.Fatalf("expected error for non-existent directory, got nil")
	}
}

// TestWriteFlush verifies that data written is correctly flushed to the log file.
func TestWriteFlush(t *testing.T) {
	tempDir := t.TempDir()
	w, err := New(tempDir)
	if err != nil {
		t.Fatalf("failed to create Writer: %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Logf("failed to close Writer: %v", err)
		}
		if err := os.RemoveAll(tempDir); err != nil {
			t.Fatalf("failed to remove temp directory: %v", err)
		}
	}()

	message := "Hello, test log!\n"
	n, err := w.Write([]byte(message))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(message) {
		t.Fatalf("expected to write %d bytes, wrote %d", len(message), n)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	logPath := filepath.Join(tempDir, "latest.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if string(data) != message {
		t.Errorf("log content mismatch: got %q, want %q", string(data), message)
	}
}

// TestRotation verifies that the Writer rotates the log file when max file size is exceeded.
func TestRotation(t *testing.T) {
	tempDir := t.TempDir()
	// Set max file size to 10 bytes to force rotation.
	w, err := New(tempDir, WithMaxFileSize(10))
	if err != nil {
		t.Fatalf("failed to create Writer: %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Logf("failed to close Writer: %v", err)
		}
		if err := os.RemoveAll(tempDir); err != nil {
			t.Fatalf("failed to remove temp directory: %v", err)
		}
	}()

	// Write an initial message.
	initial := "abc" // 3 bytes
	if _, err := w.Write([]byte(initial)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Write a second message that will trigger rotation.
	rotateMsg := "defghij" // 7 bytes; 3+7 == 10 triggers rotation.
	if _, err := w.Write([]byte(rotateMsg)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// After rotation, latest.log should contain rotateMsg.
	logPath := filepath.Join(tempDir, "latest.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read latest.log: %v", err)
	}
	if string(data) != rotateMsg {
		t.Errorf("latest.log content mismatch: got %q, want %q", string(data), rotateMsg)
	}

	// The rotated file should contain the initial message.
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to list directory: %v", err)
	}
	var rotatedFound bool
	for _, entry := range entries {
		if entry.Name() == "latest.log" {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".log") {
			rotatedFound = true
			rotatedPath := filepath.Join(tempDir, entry.Name())
			rotatedData, err := os.ReadFile(rotatedPath)
			if err != nil {
				t.Fatalf("failed to read rotated log file %q: %v", entry.Name(), err)
			}
			if string(rotatedData) != initial {
				t.Errorf("rotated file %q content mismatch: got %q, want %q", entry.Name(), string(rotatedData), initial)
			}
		}
	}
	if !rotatedFound {
		t.Errorf("expected a rotated log file but found none")
	}
}

// TestConcurrentWrites verifies that concurrent writes work correctly when WithSync is enabled.
func TestConcurrentWrites(t *testing.T) {
	tempDir := t.TempDir()
	// Use WithSync to enable internal synchronization.
	w, err := New(tempDir, WithSync())
	if err != nil {
		t.Fatalf("failed to create Writer: %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Logf("failed to close Writer: %v", err)
		}
		if err := os.RemoveAll(tempDir); err != nil {
			t.Fatalf("failed to remove temp directory: %v", err)
		}
	}()

	var wg sync.WaitGroup
	numGoroutines := 5
	writesPerGoroutine := 20
	message := "concurrent\n"
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				if _, err := w.Write([]byte(message)); err != nil {
					t.Errorf("Write failed: %v", err)
				}
				// Optionally sleep a bit to simulate work.
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}
	wg.Wait()
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify the total number of bytes written.
	logPath := filepath.Join(tempDir, "latest.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read latest.log: %v", err)
	}
	expectedBytes := numGoroutines * writesPerGoroutine * len(message)
	if len(data) != expectedBytes {
		t.Errorf("concurrent writes length mismatch: got %d bytes, want %d", len(data), expectedBytes)
	}
}
