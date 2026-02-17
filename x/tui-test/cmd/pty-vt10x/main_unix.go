//go:build unix

package main

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
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

	cmd := exec.Command(exe, "--child")
	ptmx, err := pty.Start(cmd)
	must(err)
	defer func() { _ = ptmx.Close() }()

	vt := vt10x.New(vt10x.WithSize(80, 24))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, rerr := ptmx.Read(buf)
			if n > 0 {
				_, _ = vt.Write(buf[:n])
			}
			if rerr != nil {
				return
			}
		}
	}()

	time.Sleep(300 * time.Millisecond)
	_, _ = ptmx.Write([]byte("x"))
	time.Sleep(200 * time.Millisecond)
	_, _ = ptmx.Write([]byte("q"))

	_ = cmd.Wait()
	_ = ptmx.Close()
	wg.Wait()

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
