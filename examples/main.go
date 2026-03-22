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
				chalk.Done(fmt.Sprintf("(foobar %d)", i))
			}
			chalk.Done()

			chalk.Task(0, "Doing something else")
			for i := 1; i <= 3; i++ {
				chalk.Task(1, fmt.Sprintf("Something else #%d", i))
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
