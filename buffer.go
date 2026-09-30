// Package spectator 是观战缓冲：固定容量的时间窗口缓冲、按时间取帧与回拖。
// 缺陷：按追加不裁剪（缓冲无界增长）、取帧边界跳过、回拖边界跳过且不判越界、查询线性扫全缓冲。
package spectator

import "errors"

// DelayMs 是观战延迟。
const DelayMs = 200

// RetainMs 是延迟之外额外保留的录像时长。
const RetainMs = 3000

// Capacity 是缓冲允许保留的帧数上限。
const Capacity = 512

// ErrOutOfRange 表示要回拖的时刻已经不在缓冲里。
var ErrOutOfRange = errors.New("spectator: 该时刻已经不在缓冲里")

// Frame 是一帧观战画面。
type Frame struct {
	Seq   int
	AtMs  int64
	State string
}

// Buffer 是观战缓冲。
type Buffer struct {
	frames  []Frame
	scanned int
}

// NewBuffer 建一个空缓冲。
func NewBuffer() *Buffer { return &Buffer{} }

// Push 收一帧。
// 缺陷：只追加，既不按时间窗口裁也不看 Capacity，内存随录像时长无界增长。
func (b *Buffer) Push(frame Frame) {
	b.frames = append(b.frames, frame)
}

// Buffered 是缓冲里的帧数。
func (b *Buffer) Buffered() int { return len(b.frames) }

// Scanned 是查询累计看过的帧数（规模观测）。
func (b *Buffer) Scanned() int { return b.scanned }

// View 返回观战者此刻应该看到的一帧。
// 缺陷：边界那一帧被跳过（用严格小于），而且每次都从头线性扫。
func (b *Buffer) View(nowMs int64) (Frame, bool) {
	limit := nowMs - DelayMs
	found := Frame{}
	ok := false
	for _, frame := range b.frames {
		b.scanned++
		if frame.AtMs < limit {
			found, ok = frame, true
		}
	}
	return found, ok
}

// Rewind 回拖到 targetMs，返回该时刻之后的帧（升序）。
// 缺陷：边界用严格大于、不判越界，也不做二分。
func (b *Buffer) Rewind(targetMs int64) ([]Frame, error) {
	out := make([]Frame, 0, len(b.frames))
	for _, frame := range b.frames {
		b.scanned++
		if frame.AtMs > targetMs {
			out = append(out, frame)
		}
	}
	return out, nil
}
