package main

import (
	"fmt"

	"github.com/hinshun/vt10x"
)

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
