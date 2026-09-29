// Package spectator 是观战延迟缓冲：按时间对齐画面、回拖取帧、窗口裁剪。
// 缺陷：缓冲按帧数裁剪而不是按时间，回拖边界差一，取帧也少算一帧。
package spectator

import "errors"

// DelayMs 是默认观战延迟，毫秒。
const DelayMs = 200

// RetainMs 是延迟之外额外保留的录像时长，供回拖使用。
const RetainMs = 3000

// MaxFrames 是缓冲允许保留的帧数上限。
const MaxFrames = 30

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
	delayMs  int64
	retainMs int64
	frames   []Frame
}

// NewBuffer 建一个缓冲。delayMs 是观战延迟，retainMs 是延迟之外保留的录像时长。
func NewBuffer(delayMs, retainMs int64) *Buffer {
	return &Buffer{delayMs: delayMs, retainMs: retainMs}
}

// Push 收一帧，并裁掉太旧的帧。
// 缺陷：按条数裁剪，帧率一变能回拖的时间就跟着变。
func (b *Buffer) Push(frame Frame) {
	b.frames = append(b.frames, frame)
	if len(b.frames) > MaxFrames {
		b.frames = b.frames[len(b.frames)-MaxFrames:]
	}
}

// Len 是缓冲里的帧数。
func (b *Buffer) Len() int { return len(b.frames) }

// Oldest 返回缓冲里最旧的一帧。
func (b *Buffer) Oldest() (Frame, bool) {
	if len(b.frames) == 0 {
		return Frame{}, false
	}
	return b.frames[0], true
}

// View 返回观战者此刻应该看到的一帧。
// 缺陷：边界那一帧被跳过，观战画面比口径慢一帧。
func (b *Buffer) View(nowMs int64) (Frame, bool) {
	limit := nowMs - b.delayMs
	found := Frame{}
	ok := false
	for _, frame := range b.frames {
		if frame.AtMs < limit {
			found, ok = frame, true
		}
	}
	return found, ok
}

// Rewind 回拖到 targetMs，返回该时刻之后的帧（时间升序）。
// 缺陷：不查保留窗口，边界也用严格大于，正好落在关键帧上就一帧都取不到。
func (b *Buffer) Rewind(targetMs int64) ([]Frame, error) {
	out := make([]Frame, 0, len(b.frames))
	for _, frame := range b.frames {
		if frame.AtMs > targetMs {
			out = append(out, frame)
		}
	}
	return out, nil
}
