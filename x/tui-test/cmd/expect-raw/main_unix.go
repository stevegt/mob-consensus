//go:build unix

// expect-raw is a small experiment using go-expect alone to drive a raw-mode TUI
// over a pseudo-terminal.
//
// Unlike the expect+vt10x experiment, this one does not attempt to emulate and
// scrape the full screen; it only performs expect-style reads on the raw output.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	expect "github.com/Netflix/go-expect"
	"github.com/stevegt/mob-consensus/x/tui-test/tuidemo"
)

// main either runs the demo TUI (`--child`) or runs the parent expect harness
// that starts the child and asserts on its output.
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--child" {
		if err := tuidemo.Run(); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	exe, err := os.Executable()
	must(err)

	c, err := expect.NewConsole(expect.WithDefaultTimeout(3 * time.Second))
	must(err)
	defer c.Close()

	cmd := exec.Command(exe, "--child")
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	must(cmd.Start())

	_, err = c.ExpectString("demo-tui: ready")
	must(err)

	_, _ = c.Send("q")
	_, err = c.ExpectString("demo-tui: exited")
	must(err)

	_ = cmd.Wait()
}

// must is a tiny helper for experiments: crash-fast on unexpected errors.
func must(err error) {
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
