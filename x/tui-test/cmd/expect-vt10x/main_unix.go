//go:build unix

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	expect "github.com/Netflix/go-expect"
	"github.com/hinshun/vt10x"
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

	vt := vt10x.New(vt10x.WithSize(80, 24))
	tee := io.MultiWriter(vt, io.Discard)

	c, err := expect.NewConsole(
		expect.WithStdout(tee),
		expect.WithDefaultTimeout(3*time.Second),
	)
	must(err)
	defer c.Close()

	cmd := exec.Command(exe, "--child")
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	must(cmd.Start())

	_, err = c.ExpectString("demo-tui: ready")
	must(err)

	_, _ = c.Send("x")
	_, _ = c.Send("q")
	_, err = c.ExpectString("demo-tui: exited")
	must(err)

	_ = cmd.Wait()

	vt.Lock()
	defer vt.Unlock()
	fmt.Printf("title: %q\n", vt.Title())
	fmt.Printf("screen:\n%s\n", vt.String())
}

func must(err error) {
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
