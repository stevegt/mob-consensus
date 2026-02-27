//go:build unix

// tmux-capture is an experiment that uses tmux as a terminal multiplexer to
// run a child TUI and scrape its rendered screen via `tmux capture-pane`.
//
// It uses a dedicated tmux server socket under a temp dir, so it should not
// interfere with a user's existing tmux sessions.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/stevegt/mob-consensus/x/tui-test/tuidemo"
)

// main either runs the demo TUI (`--child`) or starts a private tmux server,
// runs the child, sends a quit key, then captures and prints the pane.
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--child" {
		if err := tuidemo.Run(); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if _, err := exec.LookPath("tmux"); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "tmux not found in PATH")
		os.Exit(2)
	}

	exe, err := os.Executable()
	must(err)

	tmp, err := os.MkdirTemp("", "tui-test-tmux-")
	must(err)
	defer func() { _ = os.RemoveAll(tmp) }()

	sock := filepath.Join(tmp, "tmux.sock")
	session := "tui-test"

	// Isolate this experiment from any user tmux by using a dedicated server socket.
	tmux := func(args ...string) *exec.Cmd {
		cmd := exec.Command("tmux", append([]string{"-S", sock}, args...)...)
		cmd.Env = append(os.Environ(), "TMUX=")
		return cmd
	}

	must(tmux("new-session", "-d", "-s", session, exe, "--child").Run())
	time.Sleep(400 * time.Millisecond)

	_ = tmux("send-keys", "-t", session+":0.0", "q").Run()
	time.Sleep(200 * time.Millisecond)

	out, _ := tmux("capture-pane", "-p", "-t", session+":0.0").Output()
	_ = tmux("kill-server").Run()

	fmt.Printf("capture:\n%s\n", bytes.TrimRight(out, "\n"))
}

// must is a tiny helper for experiments: crash-fast on unexpected errors.
func must(err error) {
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
