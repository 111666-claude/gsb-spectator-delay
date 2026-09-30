package spectator

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
)

// ---- 参考实现：按 README 口径写的最直接（线性）版本，用于随机对照 ----

type referenceBuffer struct {
	frames []Frame
}

func (r *referenceBuffer) push(frame Frame) {
	r.frames = append(r.frames, frame)
	cutoff := frame.AtMs - (DelayMs + RetainMs)
	kept := r.frames[:0]
	for _, candidate := range r.frames {
		if candidate.AtMs > cutoff {
			kept = append(kept, candidate)
		}
	}
	r.frames = kept
	if len(r.frames) > Capacity {
		r.frames = r.frames[len(r.frames)-Capacity:]
	}
}

func (r *referenceBuffer) view(nowMs int64) (Frame, bool) {
	limit := nowMs - DelayMs
	var found Frame
	ok := false
	for _, frame := range r.frames {
		if frame.AtMs <= limit {
			found, ok = frame, true
		}
	}
	return found, ok
}

func (r *referenceBuffer) rewind(targetMs int64) ([]Frame, bool) {
	if len(r.frames) == 0 || targetMs < r.frames[0].AtMs {
		return nil, false
	}
	out := []Frame{}
	for _, frame := range r.frames {
		if frame.AtMs >= targetMs {
			out = append(out, frame)
		}
	}
	return out, true
}

// ---- 口径测试 ----

func TestPushTrimsToTimeWindow(t *testing.T) {
	buffer := NewBuffer()
	// 60fps 推 10 秒（625 帧，最新帧 9984ms）。
	for index := 0; index < 625; index++ {
		buffer.Push(Frame{Seq: index, AtMs: int64(index * 16)})
	}
	if buffer.Buffered() != 200 {
		t.Fatalf("3200ms 窗口应留 200 帧，实际 %d", buffer.Buffered())
	}
	oldest := buffer.frames[0]
	newest := buffer.frames[len(buffer.frames)-1]
	if oldest.AtMs != newest.AtMs-(DelayMs+RetainMs)+16 {
		t.Fatalf("窗口边界不对：oldest=%d newest=%d", oldest.AtMs, newest.AtMs)
	}
	if buffer.Buffered() > Capacity {
		t.Fatal("Buffered 不能超过 Capacity")
	}
}

func TestPushCapsAtCapacity(t *testing.T) {
	buffer := NewBuffer()
	// 帧距极小，时间窗口内容纳远超 Capacity 的帧。
	for index := 0; index < 2000; index++ {
		buffer.Push(Frame{Seq: index, AtMs: int64(index)})
	}
	if buffer.Buffered() != Capacity {
		t.Fatalf("容量上限应为 %d，实际 %d", Capacity, buffer.Buffered())
	}
	if buffer.frames[0].Seq != 2000-Capacity {
		t.Fatalf("容量裁剪应保留最新的帧，最旧 Seq=%d", buffer.frames[0].Seq)
	}
}

func TestViewIncludesBoundaryFrame(t *testing.T) {
	buffer := NewBuffer()
	for index := 0; index < 63; index++ {
		buffer.Push(Frame{Seq: index, AtMs: int64(index * 16)})
	}
	// now-DelayMs 正好落在 800ms 这一帧上，必须取到它而不是 784ms。
	frame, ok := buffer.View(1000)
	if !ok || frame.AtMs != 800 {
		t.Fatalf("取帧应含边界：got=%v ok=%v", frame.AtMs, ok)
	}
	// 再早 1ms 就只能取上一帧。
	frame, ok = buffer.View(999)
	if !ok || frame.AtMs != 784 {
		t.Fatalf("边界外应取上一帧：got=%v ok=%v", frame.AtMs, ok)
	}
	// 没有任何帧早于 now-DelayMs 时不给画面。
	if _, ok := buffer.View(DelayMs - 1); ok {
		t.Fatal("第一帧（0ms）还晚于边界，不该给出画面")
	}
}

func TestRewindIncludesBoundaryAndErrorsOutOfRange(t *testing.T) {
	buffer := NewBuffer()
	for index := 0; index < 63; index++ {
		buffer.Push(Frame{Seq: index, AtMs: int64(index * 16)})
	}
	// 目标正好落在 800ms 帧上：含边界，第一帧就是 800，共 13 帧。
	frames, err := buffer.Rewind(800)
	if err != nil {
		t.Fatalf("命中边界不该报错：%v", err)
	}
	if len(frames) != 13 || frames[0].AtMs != 800 {
		t.Fatalf("回拖应含边界：len=%d first=%d", len(frames), frames[0].AtMs)
	}
	for _, frame := range frames {
		if frame.AtMs < 800 {
			t.Fatalf("回拖帧早于目标：%+v", frame)
		}
	}
	// 目标早于最旧帧（0ms）：越界报错。
	if _, err := buffer.Rewind(-1); err != ErrOutOfRange {
		t.Fatalf("越界应返回 ErrOutOfRange，实际 %v", err)
	}
	// 目标晚于最新帧：不越界，返回空集合。
	frames, err = buffer.Rewind(10000)
	if err != nil || len(frames) != 0 {
		t.Fatalf("目标晚于最新帧应返回空集合无错：len=%d err=%v", len(frames), err)
	}
	// 空缓冲任何目标都越界。
	if _, err := NewBuffer().Rewind(0); err != ErrOutOfRange {
		t.Fatalf("空缓冲回拖应越界报错，实际 %v", err)
	}
}

func TestRewindOnEmptyBuffer(t *testing.T) {
	if _, err := NewBuffer().Rewind(0); err != ErrOutOfRange {
		t.Fatalf("空缓冲回拖必须报错：%v", err)
	}
}

func TestFrameRateInvariant(t *testing.T) {
	// 同一 DelayMs 下 60fps 与 30fps，取到的时间点相差不超过一帧（较粗帧距 33ms）。
	build := func(step int64) *Buffer {
		buffer := NewBuffer()
		for at := int64(0); at <= 10000; at += step {
			index := int(at / step)
			buffer.Push(Frame{Seq: index, AtMs: int64(index) * step})
		}
		return buffer
	}
	b60, b30 := build(16), build(33)
	// now 取在最新帧附近（更早的画面已被时间窗口裁掉）。
	for _, now := range []int64{9900, 9950, 10000} {
		f60, ok60 := b60.View(now)
		f30, ok30 := b30.View(now)
		if !ok60 || !ok30 {
			t.Fatalf("两个帧率都应有画面：%v %v", ok60, ok30)
		}
		if diff := math.Abs(float64(f60.AtMs - f30.AtMs)); diff > 33 {
			t.Fatalf("now=%d 时 60fps(%d) 与 30fps(%d) 相差 %v，超过一帧", now, f60.AtMs, f30.AtMs, diff)
		}
	}
}

func TestRepeatedQueriesAreStable(t *testing.T) {
	buffer := NewBuffer()
	for index := 0; index < 200; index++ {
		buffer.Push(Frame{Seq: index, AtMs: int64(index * 16)})
	}
	for repeat := 0; repeat < 5; repeat++ {
		frame, ok := buffer.View(2000)
		if !ok || frame.AtMs != 1792 {
			t.Fatalf("重复 View 结果不稳定：%v ok=%v", frame.AtMs, ok)
		}
		frames, err := buffer.Rewind(1600)
		if err != nil || len(frames) != 100 || frames[0].AtMs != 1600 {
			t.Fatalf("重复 Rewind 结果不稳定：len=%d err=%v", len(frames), err)
		}
	}
}

func ceilLog2(n int) int {
	if n <= 1 {
		return 0
	}
	bits, power := 0, 1
	for power < n {
		power <<= 1
		bits++
	}
	return bits
}

func TestScannedIsBinarySearchBound(t *testing.T) {
	buffer := NewBuffer()
	for index := 0; index < 625; index++ {
		buffer.Push(Frame{Seq: index, AtMs: int64(index * 16)})
	}
	buffered := buffer.Buffered()

	before := buffer.Scanned()
	_, _ = buffer.View(1000)
	if used := buffer.Scanned() - before; used > ceilLog2(buffered)+1 {
		t.Fatalf("View 扫了 %d 帧，超过 log2(%d)+1=%d", used, buffered, ceilLog2(buffered)+1)
	}

	before = buffer.Scanned()
	frames, err := buffer.Rewind(8000)
	if err != nil {
		t.Fatal(err)
	}
	if used := buffer.Scanned() - before; used > ceilLog2(buffered)+len(frames) {
		t.Fatalf("Rewind 扫了 %d 帧，超过 log2(%d)+%d", used, buffered, len(frames))
	}
}

func TestScale173kFramesAnd10kQueries(t *testing.T) {
	buffer := NewBuffer()
	const frames = 173000 // 20 分钟 144fps
	for index := 0; index < frames; index++ {
		buffer.Push(Frame{Seq: index, AtMs: int64(index) * 7})
	}
	if buffer.Buffered() > Capacity {
		t.Fatalf("17.3 万帧后 Buffered=%d 超过 Capacity=%d", buffer.Buffered(), Capacity)
	}
	for query := 0; query < 10000; query++ {
		now := int64(query*97 + 1000)
		frame, ok := buffer.View(now)
		if ok && frame.AtMs > now-DelayMs {
			t.Fatalf("View 取到未来帧：%d > %d", frame.AtMs, now-DelayMs)
		}
		if used := buffer.Scanned(); used > 0 {
			_ = used
		}
	}
	// 最后一次查询单独核对二分上界。
	before := buffer.Scanned()
	lastAt := int64(frames-1) * 7
	rewound, err := buffer.Rewind(lastAt - 500)
	if err != nil {
		t.Fatal(err)
	}
	if used := buffer.Scanned() - before; used > ceilLog2(buffer.Buffered())+len(rewound) {
		t.Fatalf("规模场景 Scanned=%d 超界", used)
	}
}

func TestMatchesReferenceOn200RandomCases(t *testing.T) {
	rng := rand.New(rand.NewSource(20261001))
	for trial := 0; trial < 200; trial++ {
		step := int64(1 + rng.Intn(40))
		count := 1 + rng.Intn(3000)
		buffer := NewBuffer()
		reference := &referenceBuffer{}
		base := rng.Int63n(5000)
		for index := 0; index < count; index++ {
			frame := Frame{Seq: index, AtMs: base + int64(index)*step, State: "s"}
			buffer.Push(frame)
			reference.push(frame)
		}
		if buffer.Buffered() != len(reference.frames) {
			t.Fatalf("trial=%d Buffered 不一致：impl=%d ref=%d", trial, buffer.Buffered(), len(reference.frames))
		}
		if !reflect.DeepEqual(buffer.frames, reference.frames) {
			t.Fatalf("trial=%d 缓冲内容不一致", trial)
		}
		last := base + int64(count-1)*step
		for query := 0; query < 16; query++ {
			now := base - DelayMs + rng.Int63n(last-base+2*DelayMs)
			gotFrame, gotOK := buffer.View(now)
			wantFrame, wantOK := reference.view(now)
			if gotOK != wantOK || (gotOK && gotFrame != wantFrame) {
				t.Fatalf("trial=%d View(%d) 不一致：impl=(%v,%v) ref=(%v,%v)",
					trial, now, gotFrame.AtMs, gotOK, wantFrame.AtMs, wantOK)
			}
			target := base - 100 + rng.Int63n(last-base+200)
			gotRewind, gotErr := buffer.Rewind(target)
			wantRewind, wantOK := reference.rewind(target)
			if (gotErr == nil) != wantOK {
				t.Fatalf("trial=%d Rewind(%d) 越界判定不一致：err=%v ref=%v", trial, target, gotErr, wantOK)
			}
			if gotErr == nil && !reflect.DeepEqual(gotRewind, wantRewind) {
				t.Fatalf("trial=%d Rewind(%d) 结果不一致：impl=%v ref=%v",
					trial, target, gotRewind, wantRewind)
			}
		}
	}
}
