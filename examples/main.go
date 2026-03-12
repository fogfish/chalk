package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/fogfish/chalk"
)

func main() {
	chalk.Start(
		func(context.Context, string, io.Reader, io.Writer) error {
			chalk.Task(0, "Configure")
			time.Sleep(2 * time.Second)
			chalk.Done()

			chalk.Task(0, "Reading")
			time.Sleep(1 * time.Second)
			chalk.Fail(fmt.Errorf("Unable to read data becuase something went wrong and we need to report it to the user given that we have a nice way to do it."))

			chalk.Task(0, "Doing something")
			for i := 1; i <= 10; i++ {
				chalk.Task(1, fmt.Sprintf("Doing #%d", i))
				time.Sleep(500 * time.Millisecond)
				chalk.Done()
			}
			chalk.Done()

			return nil
		},
	)
}
