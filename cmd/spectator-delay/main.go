// Command spectator-delay 跑观战缓冲样例。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"example.com/spectator"
)

func feed(buffer *spectator.Buffer, stepMs, untilMs int64) {
	seq := 0
	for at := int64(0); at <= untilMs; at += stepMs {
		buffer.Push(spectator.Frame{Seq: seq, AtMs: at, State: fmt.Sprintf("p%d", seq)})
		seq++
	}
}

// Run 执行一次命令行调用，返回退出码。
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("spectator-delay", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sample := flags.String("sample", "view", "view / retain / rewind / growth")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	switch *sample {
	case "view":
		// 60fps 画面，观战者停在 1000ms。
		buffer := spectator.NewBuffer()
		feed(buffer, 16, 992)
		frame, ok := buffer.View(1000)
		fmt.Fprintf(stdout, "atms=%d ok=%v\n", frame.AtMs, ok)
	case "retain":
		// 十秒 60fps 画面，回拖到 8000ms。
		buffer := spectator.NewBuffer()
		feed(buffer, 16, 9996)
		frames, err := buffer.Rewind(8000)
		if err != nil {
			fmt.Fprintf(stdout, "buffered=%d frames=0 error=true\n", buffer.Buffered())
			return 0
		}
		fmt.Fprintf(stdout, "buffered=%d frames=%d scanned=%d\n",
			buffer.Buffered(), len(frames), buffer.Scanned())
	case "rewind":
		// 回拖到 800ms，正好落在一帧上。
		buffer := spectator.NewBuffer()
		feed(buffer, 16, 992)
		frames, err := buffer.Rewind(800)
		if err != nil {
			fmt.Fprintln(stdout, "frames=0 error=true")
			return 0
		}
		fmt.Fprintf(stdout, "frames=%d first=%d\n", len(frames), frames[0].AtMs)
	case "growth":
		// 长时间观战：一百万个 16ms 的采样。
		buffer := spectator.NewBuffer()
		feed(buffer, 16, 999999*16)
		fmt.Fprintf(stdout, "buffered=%d\n", buffer.Buffered())
	default:
		fmt.Fprintln(stderr, "需要 --sample view|retain|rewind|growth")
		return 2
	}
	return 0
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
