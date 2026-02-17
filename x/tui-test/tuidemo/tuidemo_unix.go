//go:build unix

package tuidemo

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func Run() error {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("stdin is not a tty")
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("make raw: %w", err)
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	fmt.Fprintln(os.Stdout, "demo-tui: ready")
	fmt.Fprint(os.Stdout, "\x1b[?25l\x1b[2J\x1b[H")
	fmt.Fprint(os.Stdout, "tuidemo: press q to quit\r\n")
	fmt.Fprint(os.Stdout, "tuidemo: typing updates the screen\r\n")

	var b [1]byte
	for {
		n, err := os.Stdin.Read(b[:])
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		if n == 0 {
			continue
		}
		if b[0] == 'q' {
			break
		}

		fmt.Fprintf(os.Stdout, "\x1b[4;1Hlast byte: 0x%02x ('%s')      \r\n", b[0], printableByte(b[0]))
	}

	fmt.Fprint(os.Stdout, "\x1b[0m\x1b[?25h\r\n")
	fmt.Fprintln(os.Stdout, "demo-tui: exited")
	return nil
}

func printableByte(b byte) string {
	if b >= 0x20 && b <= 0x7e {
		return string([]byte{b})
	}
	return "."
}
