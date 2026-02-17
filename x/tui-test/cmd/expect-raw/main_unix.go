//go:build unix

package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	expect "github.com/Netflix/go-expect"
	"github.com/stevegt/mob-consensus/x/tui-test/tuidemo"
)

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

func must(err error) {
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
