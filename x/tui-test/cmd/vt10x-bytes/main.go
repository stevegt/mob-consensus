// vt10x-bytes is a minimal experiment that feeds raw ANSI/VT sequences into a
// vt10x emulator and prints the parsed terminal state (title, cursor, screen).
package main

import (
	"fmt"

	"github.com/hinshun/vt10x"
)

// main writes a few control sequences into vt10x and prints what vt10x thinks
// the screen looks like.
func main() {
	vt := vt10x.New(vt10x.WithSize(40, 10))
	_, _ = vt.Write([]byte("\x1b]0;vt10x-bytes\x07"))
	_, _ = vt.Write([]byte("\x1b[2J\x1b[HHello\r\nWorld"))

	vt.Lock()
	defer vt.Unlock()

	c := vt.Cursor()
	fmt.Printf("title: %q\n", vt.Title())
	fmt.Printf("cursor: (%d,%d) visible=%v\n", c.X, c.Y, vt.CursorVisible())
	fmt.Printf("screen:\n%s\n", vt.String())
}
