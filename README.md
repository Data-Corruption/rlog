# rlog

[![Go Reference](https://pkg.go.dev/badge/github.com/Data-Corruption/rlog.svg)](https://pkg.go.dev/github.com/Data-Corruption/rlog)

`rlog` implements a low-overhead high-performance writer for logging with automatic file rotation, which plugs neatly into Go's standard `log.Logger`.

## Features

- Buffered writes to reduce disk I/O overhead
- Automatic flushing based on buffer size or age
- File rotation with sub-second timestamp precision
- Optional synchronization for concurrent writes

## Installation

```sh
go get github.com/Data-Corruption/rlog
```

## Usage

### Using `rlog.Writer`

The core `rlog.Writer` provides buffered I/O with rotation.

```go
package main

import (
    "log"
    "github.com/Data-Corruption/rlog"
)

func main() {
  // Create a writer in the "logs" directory with a 1 KB buffer
  // Directory must exist beforehand.
  w, err := rlog.New("logs", rlog.WithMaxBufSize(1024))
  if err != nil {
    log.Fatalf("Failed to create log writer: %v", err)
  }
  // Close should be deferred to ensure buffer is flushed and file is closed on exit.
  defer w.Close()

  _, err = w.Write([]byte("Hello, log file!\n"))
  if err != nil {
    log.Printf("Write failed: %v", err)
  }
  // Flush is optional here; writing often triggers flushing anyway based on size/age.
  // err = w.Flush()
  // if err != nil {
  //  log.Printf("Flush failed: %v", err)
  // }
}
```

**Configuration Options**:

| Option           | Default | Description |
|------------------|---------|-------------|
| `WithMaxFileSize` | 256 MB | Maximum size of output files |
| `WithMaxBufSize`  | 4 KB | Maximum size of the buffer before flushing |
| `WithMaxBufAge`   | 15 sec | Maximum age of the buffer before flushing |
| `WithSync`        | false   | Enable thread-safe writes |

**Important Notes for rlog.Writer**:

- **Age-Based Flushing**: The buffer is only checked for flushing due to `WithMaxBufAge` during a `Write` operation. If your application has periods of inactivity longer than the `maxBufAge` but you still want logs flushed periodically, you must implement a separate goroutine that calls `w.Flush()` on a timer.
- **Error Handling**: If any operation (`Write`, `Flush`, `Close`, internal rotation) encounters an error, that error is stored internally. Subsequent calls to these methods will return the first error encountered. Check errors on all operations, including `Close`.
- **Concurrency**: The `rlog.Writer` is not safe for concurrent use by default. If multiple goroutines will call `Write`, `Flush`, or `Close` on the same writer instance, you must use the `rlog.WithSync()` option during creation.


### Using `rlog.Writer` with `log.Logger`

`rlog.Writer` implements `io.Writer`, making it easy to use with Go's standard `log.Logger`.

```go
package main

import (
  "log"
  "github.com/Data-Corruption/rlog"
)

func main() {
  // Standard log.Logger serializes writes, so WithSync() is generally not needed here.
  logWriter, err := rlog.New("logs")
  if err != nil {
    log.Fatalf("Failed to create log writer: %v", err)
  }
  defer logWriter.Close() // Ensure logs are flushed on exit

  // Configure standard logger to use rlog.Writer
  logger := log.New(logWriter, "myapp: ", log.LstdFlags|log.Lshortfile)

  logger.Println("Application started.")
  logger.Printf("Processed %d records.", 100)
}
```

### Using the `rlog/logger` Package

If you need a simple, leveled logger built on top of `rlog`, use the `github.com/Data-Corruption/rlog/logger` subpackage. It provides Debug, Info, Warn, Error, and None levels and manages the underlying rlog.Writer automatically.

```go
package main

import (
  "context"
  "log"
  "github.com/Data-Corruption/rlog/logger"
)

func main() {
  // Create a logger. Directory will be created if it doesn't exist.
  // Level can be "debug", "info", "warn", "error", or "none".
  l, err := logger.New("./app_logs", "debug") // Log debug and above
  if err != nil {
    log.Fatalf("Failed to create logger: %v", err)
  }
  defer l.Close() // Ensure logs are flushed

  // --- Direct Logging Methods ---
  l.Info("Application starting...")
  l.Debugf("Configuration value: %s", "some_value")
  l.Warn("Potential issue detected.")
  l.Error("An error occurred!", err) // Example logging an error variable

  // --- Context-Based Logging ---
  // Useful when passing the logger explicitly through function calls is cumbersome.
  ctx := context.Background()
  ctx = logger.IntoContext(ctx, l) // Place logger into context

  doSomething(ctx)
}

func doSomething(ctx context.Context) {
  // Retrieve logger from context and log
  logger.Info(ctx, "Doing something important.")
  if warningCondition := true; warningCondition {
    logger.Warn(ctx, "Warning during operation.")
  }
}
```

## License

Mozilla Public License, version 2.0. See [LICENSE](./LICENSE.md) for details.
