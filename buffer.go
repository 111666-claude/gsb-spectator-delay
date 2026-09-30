// Package spectator 是观战缓冲：固定容量的时间窗口缓冲、按时间取帧与回拖。
package spectator

import (
	"errors"
	"sort"
)

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
func (b *Buffer) Push(frame Frame) {
	b.frames = append(b.frames, frame)
	// 时间窗口裁：窗口为 (最新帧.AtMs-(DelayMs+RetainMs), 最新帧.AtMs]，
	// 恰好覆盖 DelayMs+RetainMs 的时长（边界上最旧那一帧超出窗口，裁掉）。
	cutoff := frame.AtMs - (DelayMs + RetainMs)
	drop := sort.Search(len(b.frames), func(i int) bool {
		return b.frames[i].AtMs > cutoff
	})
	// 容量裁：时间窗口内也只保留最新的 Capacity 帧。
	if kept := len(b.frames) - drop; kept > Capacity {
		drop += kept - Capacity
	}
	if drop > 0 {
		copy(b.frames, b.frames[drop:])
		b.frames = b.frames[:len(b.frames)-drop]
	}
}

// Buffered 是缓冲里的帧数。
func (b *Buffer) Buffered() int { return len(b.frames) }

// Scanned 是查询累计看过的帧数（规模观测）。
func (b *Buffer) Scanned() int { return b.scanned }

// View 返回观战者此刻应该看到的一帧。
func (b *Buffer) View(nowMs int64) (Frame, bool) {
	limit := nowMs - DelayMs
	idx := b.upperBound(limit) - 1
	if idx < 0 {
		return Frame{}, false
	}
	return b.frames[idx], true
}

// Rewind 回拖到 targetMs，返回该时刻之后的帧（升序）。
func (b *Buffer) Rewind(targetMs int64) ([]Frame, error) {
	if len(b.frames) == 0 || targetMs < b.frames[0].AtMs {
		return nil, ErrOutOfRange
	}
	idx := b.lowerBound(targetMs)
	out := make([]Frame, len(b.frames)-idx)
	copy(out, b.frames[idx:])
	return out, nil
}

// upperBound 返回首个 AtMs 大于 limit 的下标（AtMs 升序），按时间二分。
func (b *Buffer) upperBound(limit int64) int {
	lo, hi := 0, len(b.frames)
	for lo < hi {
		mid := lo + (hi-lo)/2
		b.scanned++
		if b.frames[mid].AtMs > limit {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

// lowerBound 返回首个 AtMs 不小于 target 的下标（AtMs 升序），按时间二分。
func (b *Buffer) lowerBound(target int64) int {
	lo, hi := 0, len(b.frames)
	for lo < hi {
		mid := lo + (hi-lo)/2
		b.scanned++
		if b.frames[mid].AtMs >= target {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}
