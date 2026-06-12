# chalk

Task-oriented I/O helper for Golang CLI. It wires together flag-driven input/output routing (local filesystem, S3, stdin/stdout) with a live terminal progress UI so you can focus on the processing logic.

## Features

- Resolves input from a local directory (`-I ./path`), an S3 bucket (`-I s3://bucket`), explicit file arguments, or stdin
- Resolves output to a local directory (`-O ./path`), an S3 bucket (`-O s3://bucket`), a single file (`-o out.txt`), or stdout
- Renders a live progress tree with elapsed timers, spinner, and per-task pass/fail status

## Quick Start

```go
package main

import (
  "context"
  "io"

  "github.com/fogfish/chalk"
  "github.com/fogfish/chalk/rt/cli"
)

func process(ctx context.Context, path string, r io.Reader, w io.Writer) error {
  chalk.Task(ctx, path)

  // Subtask at level 1
  chalk.Task(chalk.Sub(ctx), "read")
  // ... read from fs
  chalk.Done()

  chalk.Task(chalk.Sub(ctx), "transform")
  // ... do work
  chalk.Done()

  chalk.Done()
  return nil
}

func main() {
  cli.Start(process)
}
```

Run it:

```
mytool -I ./input -O ./output
mytool -I s3://my-bucket -O s3://out-bucket
cat file.txt | mytool
```

## License

See [LICENSE](LICENSE).
