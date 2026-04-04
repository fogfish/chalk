//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

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
		func(ctx context.Context, _ string, _ io.Reader, _ io.Writer) error {
			chalk.Task(ctx, "Configure")
			time.Sleep(2 * time.Second)
			chalk.Done()

			chalk.Task(ctx, "Reading")
			time.Sleep(1 * time.Second)
			chalk.Fail(fmt.Errorf("unable to read data becuase something went wrong and we need to report it to the user given that we have a nice way to do it"))

			chalk.Task(ctx, "Doing something")
			for i := 1; i <= 10; i++ {
				chalk.Task(chalk.Sub(ctx), fmt.Sprintf("Doing #%d", i))
				time.Sleep(500 * time.Millisecond)
				chalk.Done(fmt.Sprintf("(foobar %d)", i))
			}
			chalk.Done()

			chalk.Task(ctx, "Doing something else")
			for i := 1; i <= 3; i++ {
				chalk.Task(chalk.Sub(ctx), fmt.Sprintf("Something else #%d", i))
				time.Sleep(500 * time.Millisecond)
				chalk.Printf("Some long text just to explain very complex operations with #%d. The text is reported to the user given that we have a nice way to do it using multiple lines.", i)
				chalk.Done()
			}
			chalk.Done()

			chalk.Printf("\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\nComplete!\n\n")

			return nil
		},
	)
}
