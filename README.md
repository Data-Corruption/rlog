# rlog

[![Go Reference](https://pkg.go.dev/badge/github.com/Data-Corruption/rlog.svg)](https://pkg.go.dev/github.com/Data-Corruption/rlog)

`rlog` implements a low-overhead high-performance writer for logging with automatic file rotation, which plugs neatly into Go's standard `log.Logger`.

## Features

- Buffered writes to reduce disk I/O overhead
- Automatic flushing based on buffer size or age
- File rotation with sub-second timestamp precision
- Optional synchronization for concurrent writes
- Pair well with Go's standard `log.Logger`

## Installation

```sh
go get github.com/Data-Corruption/rlog
```

## Usage

### Basic Example

```go
w, err := rlog.New("logs", rlog.WithMaxBufSize(1024)) // 1 KB buffer
if err != nil {
    log.Fatalf("Failed to create log writer: %v", err)
}
defer w.Close()

w.Write([]byte("Hello, log file!\n"))
w.Flush() // Optional, forces buffer flush
```

### Using `rlog` with `log.Logger`

```go
package main

import (
  "log"
  "github.com/Data-Corruption/rlog"
)

func main() {
  // log.Logger serializes writes, WithSync() not needed.
  logWriter, err := rlog.New("logs")
  if err != nil {
    log.Fatalf("Failed to create log writer: %v", err)
  }
  defer logWriter.Close()
  logger := log.New(logWriter, "", log.LstdFlags)
  logger.Println("Application started.")
}
```

## Configuration Options

| Option           | Default | Description |
|------------------|---------|-------------|
| `WithMaxFileSize` | 256 MB | Maximum size of output files |
| `WithMaxBufSize`  | 4 KB | Maximum size of the buffer before flushing |
| `WithMaxBufAge`   | 15 sec | Maximum age of the buffer before flushing |
| `WithSync`        | false   | Enable thread-safe writes |

## License

Mozilla Public License, version 2.0. See [LICENSE](./LICENSE.md) for details.
