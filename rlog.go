// Copyright 2025 Matthew Pombo. All rights reserved.
// Use of this source code is governed by an Mozilla Public License, version 2.0
// license that can be found in the LICENSE file.

// Package rlog provides a buffered log writer with automatic flushing and file
// rotation. It is designed for high-frequency logging scenarios where the
// overhead of file I/O should be minimized. The Writer type can be safe for
// concurrent use and plugs neatly into the standard log.Logger type.
//
// The Writer type implements io.Writer and writes data to a file within a
// specified directory. Flushes occur during Write() calls where the buffer
// exceeds a configurable size or age. Rotations occur when the latest log file
// exceeds a maximum size. Rotation, renames the latest log file ("latest.log")
// to a timestamp (with sub-second resolution) and a new "latest.log" is created.
//
// Note the Writer will not automatically flush when the buffer age exceeds the
// maximum buffer age. If you want that functionality, you should create a
// separate goroutine that calls Flush() periodically.
//
// Note that by default Writer is not safe for concurrent use. Use the WithSync
// option to enable internal synchronization.
//
// Usage:
//
//	// Create a new synchronized Writer with a maximum file size of 1 GB.
//	w, err := rlog.New("logs", rlog.WithMaxFileSize(1024*1024), rlog.WithSync())
//	if err != nil {
//		log.Fatalf("Failed to create log writer: %v", err)
//	}
//	w.Write([]byte("Hello, log file!"))
//	w.Flush() // Optionally manually flush the buffer.
//	w.Close() // Flush and close file on program exit.
//
// Common Use Case (with the standard log package):
//
//	var (
//		logWriter *rlog.Writer
//		logger    *log.Logger
//	)
//
//	func main() {
//		var err error
//		logWriter, err = rlog.New("logs") // WithSync unnecessary as log.Logger serializes writes
//		if err != nil {
//			log.Fatalf("Failed to create log writer: %v", err)
//		}
//		// Create a new logger with the log writer as the output.
//		logger = log.New(logWriter, "", 0)
//		logger.Println("Hello, log file!")
//		logWriter.Close() // Flush and close file on program exit.
//	}
package rlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	DefaultMaxFileSize = 256 * 1024 * 1024 // 256 MB
	DefaultMaxBufSize  = 4096              // 4 KB
	DefaultMaxBufAge   = 15 * time.Second  // 15 seconds
)

type noCopy struct{} // see https://github.com/golang/go/issues/8005#issuecomment-190753527

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

// Writer implements buffered log writing with automatic file rotation.
// If a write operation returns an error, no further data is accepted and subsequent
// function calls will return the error.
type Writer struct {
	noCopy noCopy

	mu        *sync.Mutex // pointer to allow disabling synchronization using nil
	err       error
	buf       []byte
	file      *os.File
	dirPath   string
	lastFlush time.Time

	maxFileSize int64
	maxBufSize  int
	maxBufAge   time.Duration
}

// New creates and initializes a new Writer for the specified directory.
// The directory must exist. Additional options can be provided to customize
// the Writer's behavior.
func New(dirPath string, opts ...Option) (*Writer, error) {
	if fi, err := os.Stat(dirPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory %q does not exist", dirPath)
		} else {
			return nil, err
		}
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("path %q is not a directory", dirPath)
	}
	w := &Writer{
		buf:         make([]byte, 0, DefaultMaxBufSize),
		dirPath:     dirPath,
		lastFlush:   time.Now(),
		maxFileSize: DefaultMaxFileSize,
		maxBufSize:  DefaultMaxBufSize,
		maxBufAge:   DefaultMaxBufAge,
	}
	for _, opt := range opts {
		opt(w)
	}
	var err error
	if w.file, err = os.OpenFile(filepath.Join(w.dirPath, "latest.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err != nil {
		return nil, err
	}
	return w, nil
}

// options

// Option defines a function that configures a Writer.
type Option func(*Writer)

// WithMaxFileSize sets the maximum size of the log file before it's rotated.
func WithMaxFileSize(size int64) Option {
	return func(w *Writer) {
		w.maxFileSize = size
	}
}

// WithMaxBufSize sets the maximum size of the internal buffer before flushing.
func WithMaxBufSize(size int) Option {
	return func(w *Writer) {
		w.maxBufSize = size
		if cap(w.buf) < size {
			newBuf := make([]byte, len(w.buf), size)
			copy(newBuf, w.buf) // in case there's data in the buffer
			w.buf = newBuf
		}
	}
}

// WithMaxBufAge sets the age of the buffer. On Write(), if the buffer is older
// than maxBufAge, it will be flushed.
func WithMaxBufAge(d time.Duration) Option {
	return func(w *Writer) {
		w.maxBufAge = d
	}
}

// WithSync configures the Writer to be safe for concurrent use by enabling
// internal synchronization via a mutex.
func WithSync() Option {
	return func(w *Writer) {
		w.mu = &sync.Mutex{}
	}
}

// methods

// Flush writes any buffered data to disk. Flushing happens automatically during Write()
// when the buffer exceeds maxBufSize or maxBufAge. Manually flushing is usually unnecessary.
func (w *Writer) Flush() error {
	if w.mu != nil {
		w.mu.Lock()
		defer w.mu.Unlock()
	}
	if w.err != nil {
		return w.err
	}
	return w.flush()
}

// Write appends the contents of p to the Writer's buffer.
// When the buffer's size exceeds maxBufSize or the time since the last flush
// exceeds maxBufAge, the buffer is flushed to disk.
//
// Write implements the io.Writer interface and returns the length of p on success.
// Partial writes are not supported.
func (w *Writer) Write(p []byte) (int, error) {
	if w.mu != nil {
		w.mu.Lock()
		defer w.mu.Unlock()
	}
	if w.err != nil {
		return 0, w.err
	}
	w.buf = append(w.buf, p...)
	if len(w.buf) >= w.maxBufSize || time.Since(w.lastFlush) >= w.maxBufAge {
		if err := w.flush(); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// WriteString is a convenience method that wraps Write() for string data.
func (w *Writer) WriteString(s string) (int, error) {
	bytes := []byte(s)
	return w.Write(bytes)
}

// Close flushes any remaining buffered data to disk and closes the underlying file.
// It should be called when the Writer is no longer needed.
func (w *Writer) Close() error {
	if w.mu != nil {
		w.mu.Lock()
		defer w.mu.Unlock()
	}
	if w.err != nil {
		return w.err
	}
	if err := w.flush(); err != nil {
		return err
	}
	return w.file.Close()
}

// internal methods

// flush writes the contents of the buffer to the latest log file.
// If writing the buffer would cause the file to exceed maxFileSize,
// the file is rotated before writing. After a successful flush, the buffer
// is reset and lastFlush is updated.
//
// flush returns an error if the write, file sync, or rotation fails.
func (w *Writer) flush() error {
	if w.err != nil {
		return w.err
	}
	if w.file == nil {
		w.err = fmt.Errorf("log file %q is closed", filepath.Join(w.dirPath, "latest.log"))
		return w.err
	}
	if len(w.buf) == 0 {
		return nil
	}
	// Determine if the file needs to be rotated.
	fi, err := w.file.Stat()
	if err != nil {
		w.err = fmt.Errorf("failed to stat log file: %v", err)
		return w.err
	}
	if fi.Size()+int64(len(w.buf)) >= w.maxFileSize {
		if err := w.rotate(); err != nil {
			return err
		}
	}
	// Write the buffer to the file and sync.
	if _, err := w.file.Write(w.buf); err != nil {
		w.err = fmt.Errorf("failed to write to log file: %v", err)
		return w.err
	}
	if err := w.file.Sync(); err != nil {
		w.err = fmt.Errorf("failed to sync log file: %v", err)
		return w.err
	}
	w.buf = w.buf[:0]
	w.lastFlush = time.Now()
	return nil
}

// rotate renames the latest log file with a timestamp and creates a new
// "latest.log" file for subsequent writes. The timestamp includes sub-second
// precision to avoid naming collisions in high-frequency rotation scenarios.
func (w *Writer) rotate() error {
	if w.err != nil {
		return w.err
	}
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			w.err = fmt.Errorf("failed to close log file: %v", err)
			return w.err
		}
		w.file = nil
	}
	oldPath := filepath.Join(w.dirPath, "latest.log")
	ts := time.Now().Format("20060102-150405.000000")
	newPath := filepath.Join(w.dirPath, fmt.Sprintf("%s.log", ts))
	if err := os.Rename(oldPath, newPath); err != nil {
		w.err = fmt.Errorf("failed to rename log file: %v", err)
		return err
	}
	var err error
	if w.file, err = os.OpenFile(oldPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err != nil {
		w.err = fmt.Errorf("failed to create new log file: %v", err)
		return err
	}
	return nil
}
