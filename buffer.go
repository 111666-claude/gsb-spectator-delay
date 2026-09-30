// Package spectator 是观战缓冲：固定容量的时间窗口缓冲、按时间取帧与回拖。
package spectator

import (
	"errors"
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

// Buffer 是观战缓冲：定容环形存储，内存只跟 Capacity 有关。
type Buffer struct {
	ring    [Capacity]Frame
	head    int // 最旧帧在 ring 里的下标
	count   int // 缓冲帧数
	scanned int
}

// NewBuffer 建一个空缓冲。
func NewBuffer() *Buffer { return &Buffer{} }

func (b *Buffer) at(index int) Frame { return b.ring[(b.head+index)%Capacity] }

// Push 收一帧。
// 只保留时间窗口 (最新帧.AtMs-(DelayMs+RetainMs), 最新帧] 里的帧，
// 且帧数不超过 Capacity。
func (b *Buffer) Push(frame Frame) {
	if b.count > 0 {
		// 时间窗口后半开：只留 AtMs > 最新帧.AtMs - (DelayMs + RetainMs)。
		cutoff := frame.AtMs - (DelayMs + RetainMs)
		drop := 0
		for drop < b.count && b.at(drop).AtMs <= cutoff {
			drop++
		}
		b.head = (b.head + drop) % Capacity
		b.count -= drop
	}
	if b.count == Capacity {
		b.head = (b.head + 1) % Capacity
	} else {
		b.count++
	}
	b.ring[(b.head+b.count-1)%Capacity] = frame
}

// Buffered 是缓冲里的帧数。
func (b *Buffer) Buffered() int { return b.count }

// Scanned 是查询累计看过的帧数（规模观测）。
func (b *Buffer) Scanned() int { return b.scanned }

// View 返回观战者此刻应该看到的一帧。
// 取 AtMs <= now - DelayMs 的最新一帧（含边界），按时间二分定位。
func (b *Buffer) View(nowMs int64) (Frame, bool) {
	limit := nowMs - DelayMs
	if b.count == 0 {
		return Frame{}, false
	}
	// 二分第一个 AtMs > limit 的下标：它前面的一帧即所求（含边界）。
	lo, hi := 0, b.count
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		b.scanned++
		if b.at(mid).AtMs > limit {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	if lo == 0 {
		return Frame{}, false
	}
	return b.at(lo - 1), true
}

// Rewind 回拖到 targetMs，返回该时刻之后的帧（升序）。
// 返回所有 AtMs >= targetMs 的帧（含边界，升序）；
// targetMs 早于缓冲里最旧一帧时返回 ErrOutOfRange。按时间二分定位。
func (b *Buffer) Rewind(targetMs int64) ([]Frame, error) {
	if b.count == 0 {
		return nil, ErrOutOfRange
	}
	// 二分第一个 AtMs >= targetMs 的下标（含边界）。
	// 探到最旧帧时一并判定越界，不额外增加探测次数。
	lo, hi := 0, b.count
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		b.scanned++
		oldest := b.at(mid)
		if mid == 0 && oldest.AtMs > targetMs {
			return nil, ErrOutOfRange
		}
		if oldest.AtMs < targetMs {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return b.copyFrom(lo), nil
}

// copyFrom 返回逻辑下标 from 起的所有帧（升序），并计入命中帧数。
func (b *Buffer) copyFrom(from int) []Frame {
	out := make([]Frame, 0, b.count-from)
	for index := from; index < b.count; index++ {
		b.scanned++
		out = append(out, b.at(index))
	}
	return out
}
