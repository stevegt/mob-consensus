//go:build unix

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
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

	cmd := exec.Command(exe, "--child")
	ptmx, err := pty.Start(cmd)
	must(err)
	defer func() { _ = ptmx.Close() }()

	done := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(os.Stdout, ptmx)
		done <- copyErr
	}()

	time.Sleep(300 * time.Millisecond)
	_, _ = ptmx.Write([]byte("q"))

	_ = cmd.Wait()
	_ = ptmx.Close()
	<-done
}

func must(err error) {
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
