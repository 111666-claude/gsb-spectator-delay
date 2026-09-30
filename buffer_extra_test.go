package spectator

import (
	"math"
	"math/rand"
	"testing"
)

// feedN 按 stepMs 间隔从 0 开始压入 n 帧。
func feedN(b *Buffer, stepMs int64, n int) {
	for index := 0; index < n; index++ {
		b.Push(Frame{Seq: index, AtMs: int64(index) * stepMs})
	}
}

func TestTrimsByTimeWindow(t *testing.T) {
	// 十秒 60fps（16ms 步进）：窗口半开 (newest-3200, newest]。
	buffer := NewBuffer()
	feedN(buffer, 16, 625)
	if got := buffer.Buffered(); got != 200 {
		t.Fatalf("窗口裁剪后应为 200 帧，实际 %d", got)
	}
	// 最旧帧必须严格晚于 newest-(DelayMs+RetainMs)，且新窗口外的旧帧被丢。
	cutoff := buffer.at(buffer.count-1).AtMs - (DelayMs + RetainMs)
	for index := 0; index < buffer.count; index++ {
		if buffer.at(index).AtMs <= cutoff {
			t.Fatalf("窗口里残留该裁掉的帧：%+v", buffer.at(index))
		}
	}
	// 短录像不裁。
	short := NewBuffer()
	feedN(short, 16, 5)
	if short.Buffered() != 5 {
		t.Fatalf("短录像不该被裁：%d", short.Buffered())
	}
}

func TestCapacityBound(t *testing.T) {
	// 高密度帧：时间窗口里塞不下时，帧数也永远不超过 Capacity。
	buffer := NewBuffer()
	for index := 0; index < 200000; index++ {
		buffer.Push(Frame{Seq: index, AtMs: int64(index)})
		if buffer.Buffered() > Capacity {
			t.Fatalf("Buffered=%d 超过 Capacity=%d", buffer.Buffered(), Capacity)
		}
	}
	if buffer.Buffered() != Capacity {
		t.Fatalf("高密度长录像应稳定在 Capacity：%d", buffer.Buffered())
	}
}

func TestViewBoundaryInclusive(t *testing.T) {
	buffer := NewBuffer()
	feedN(buffer, 16, 100)
	// now-Delay 正好落在一帧上：必须取到该帧本身（含边界）。
	frame, ok := buffer.View(800 + DelayMs)
	if !ok {
		t.Fatal("命中边界帧时必须给出画面")
	}
	if frame.AtMs != 800 {
		t.Fatalf("应取到 800ms 的边界帧，实际 %d", frame.AtMs)
	}
	// 再早 1ms：上一帧（784）。
	frame, _ = buffer.View(800 + DelayMs - 1)
	if frame.AtMs != 784 {
		t.Fatalf("应取到 784ms，实际 %d", frame.AtMs)
	}
	// 最旧帧都还晚于 now-Delay：无帧可取。
	if _, ok := buffer.View(0); ok {
		t.Fatal("没有满足 AtMs <= now-Delay 的帧时不该给出画面")
	}
}

func TestRewindBoundaryAndRange(t *testing.T) {
	buffer := NewBuffer()
	feedN(buffer, 16, 100)
	// 目标正好落在一帧上：该帧必须在结果里（含边界）。
	frames, err := buffer.Rewind(800)
	if err != nil {
		t.Fatalf("命中边界不该报错：%v", err)
	}
	if len(frames) == 0 || frames[0].AtMs != 800 {
		t.Fatalf("回拖必须包含 800ms 边界帧")
	}
	for _, frame := range frames {
		if frame.AtMs < 800 {
			t.Fatalf("回拖帧早于目标时刻：%+v", frame)
		}
	}
	// 目标早于最旧帧：ErrOutOfRange。
	if _, err := buffer.Rewind(-1); err != ErrOutOfRange {
		t.Fatalf("越界必须返回 ErrOutOfRange，实际 %v", err)
	}
	if _, err := NewBuffer().Rewind(0); err != ErrOutOfRange {
		t.Fatalf("空缓冲回拖必须返回 ErrOutOfRange，实际 %v", err)
	}
	// 目标晚于最新帧：合法但结果为空。
	if frames, err := buffer.Rewind(100000); err != nil || len(frames) != 0 {
		t.Fatalf("目标晚于最新帧应返回空结果无错误，实际 frames=%d err=%v", len(frames), err)
	}
}

func TestScannedWithinBinaryBudget(t *testing.T) {
	buffer := NewBuffer()
	feedN(buffer, 16, 625) // 窗口后 200 帧
	for _, target := range []int64{6800, 8000, 9000, 9984} {
		buffer.scanned = 0
		frames, err := buffer.Rewind(target)
		if err != nil {
			t.Fatalf("Rewind(%d)：%v", target, err)
		}
		budget := int(math.Ceil(math.Log2(float64(buffer.Buffered())))) + len(frames)
		if buffer.Scanned() > budget {
			t.Fatalf("Rewind(%d) Scanned=%d 超过预算 %d", target, buffer.Scanned(), budget)
		}
	}
	for _, now := range []int64{7000, 8000 + DelayMs, 10184} {
		buffer.scanned = 0
		if _, ok := buffer.View(now); !ok {
			t.Fatalf("View(%d) 无结果", now)
		}
		budget := int(math.Ceil(math.Log2(float64(buffer.Buffered())))) + 1
		if buffer.Scanned() > budget {
			t.Fatalf("View(%d) Scanned=%d 超过预算 %d", now, buffer.Scanned(), budget)
		}
	}
}

func TestDeterministicRepeatedQuery(t *testing.T) {
	buffer := NewBuffer()
	feedN(buffer, 16, 300)
	// 300 帧 16ms：窗口约 [1600,4784]，查询时刻必须落在窗口内。
	first, _ := buffer.View(4000)
	firstRewind, err := buffer.Rewind(2000)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 10; attempt++ {
		frame, _ := buffer.View(4000)
		if frame != first {
			t.Fatalf("重复 View 结果不一致：%+v != %+v", frame, first)
		}
		frames, err := buffer.Rewind(2000)
		if err != nil || len(frames) != len(firstRewind) || frames[0] != firstRewind[0] {
			t.Fatalf("重复 Rewind 结果不一致：%+v != %+v", frames, firstRewind)
		}
	}
}

func TestFrameRateInvariance(t *testing.T) {
	// 同一 DelayMs 下 60fps 与 30fps 取到的时间点相差不超过一帧（按较粗帧率 33ms 计）。
	sixty := NewBuffer()
	thirty := NewBuffer()
	const duration = int64(10000)
	for at := int64(0); at <= duration; at += 16 {
		sixty.Push(Frame{AtMs: at})
	}
	for at := int64(0); at <= duration; at += 33 {
		thirty.Push(Frame{AtMs: at})
	}
	// 从两边都稳定有帧的时刻开始查（跳过窗口起点的有无差异）。
	for now := int64(DelayMs + 100); now <= duration; now += 37 {
		a, aok := sixty.View(now)
		c, cok := thirty.View(now)
		if !aok || !cok {
			continue
		}
		{
			diff := a.AtMs - c.AtMs
			if diff < -33 || diff > 33 {
				t.Fatalf("now=%d：60fps=%d 与 30fps=%d 相差 %dms，超过一帧", now, a.AtMs, c.AtMs, diff)
			}
		}
	}
}

// ---- 参考实现：按时间窗口裁剪 + 含边界二分取帧 ----

type refBuffer struct {
	frames []Frame
}

func (r *refBuffer) push(f Frame) {
	cutoff := f.AtMs - (DelayMs + RetainMs)
	kept := r.frames[:0]
	for _, existing := range r.frames {
		if existing.AtMs > cutoff {
			kept = append(kept, existing)
		}
	}
	r.frames = append(kept, f)
	if len(r.frames) > Capacity {
		r.frames = r.frames[len(r.frames)-Capacity:]
	}
}

// lowerBound 返回第一个 AtMs >= target 的下标。
func (r *refBuffer) lowerBound(target int64) int {
	lo, hi := 0, len(r.frames)
	for lo < hi {
		mid := (lo + hi) / 2
		if r.frames[mid].AtMs < target {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

func (r *refBuffer) view(now int64) (Frame, bool) {
	limit := now - DelayMs
	idx := r.lowerBound(limit + 1) // 第一个 > limit
	if idx == 0 {
		return Frame{}, false
	}
	return r.frames[idx-1], true
}

func (r *refBuffer) rewind(target int64) ([]Frame, bool) {
	idx := r.lowerBound(target)
	if idx == 0 && (len(r.frames) == 0 || r.frames[0].AtMs > target) {
		return nil, false
	}
	out := make([]Frame, len(r.frames)-idx)
	copy(out, r.frames[idx:])
	return out, true
}

func TestDifferentialAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewSource(20260930))
	for caseIndex := 0; caseIndex < 200; caseIndex++ {
		step := []int64{7, 8, 16, 17, 25, 33, 34, 42, 60}[rng.Intn(9)]
		frames := rng.Intn(2000) + 1
		jitter := rng.Intn(4) - 1 // -1..2ms 抖动，时间戳仍单调不减

		actual := NewBuffer()
		reference := &refBuffer{}
		at := int64(0)
		for index := 0; index < frames; index++ {
			at += step + int64(jitter)
			frame := Frame{Seq: index, AtMs: at}
			actual.Push(frame)
			reference.push(frame)
		}

		if actual.Buffered() != len(reference.frames) {
			t.Fatalf("case %d: Buffered=%d 参考=%d", caseIndex, actual.Buffered(), len(reference.frames))
		}
		if actual.Buffered() > Capacity {
			t.Fatalf("case %d: Buffered 超过 Capacity", caseIndex)
		}

		newest := at
		oldest := reference.frames[0].AtMs
		query := func(target int64) {
			av, aok := actual.View(target)
			rv, rok := reference.view(target)
			if aok != rok || (aok && av != rv) {
				t.Fatalf("case %d View(%d): 实际=(%+v,%v) 参考=(%+v,%v)",
					caseIndex, target, av, aok, rv, rok)
			}
			af, aerr := actual.Rewind(target)
			rf, rok := reference.rewind(target)
			if aerr != nil && aerr != ErrOutOfRange {
				t.Fatalf("case %d Rewind(%d): 非预期错误 %v", caseIndex, target, aerr)
			}
			if (aerr == ErrOutOfRange) != !rok {
				t.Fatalf("case %d Rewind(%d): 越界判定不一致 err=%v 参考ok=%v",
					caseIndex, target, aerr, rok)
			}
			if aerr == nil {
				if len(af) != len(rf) {
					t.Fatalf("case %d Rewind(%d): 帧数 %d != 参考 %d",
						caseIndex, target, len(af), len(rf))
				}
				for index := range rf {
					if af[index] != rf[index] {
						t.Fatalf("case %d Rewind(%d): 第 %d 帧 %+v != %+v",
							caseIndex, target, index, af[index], rf[index])
					}
				}
				for _, f := range af {
					if f.AtMs < target {
						t.Fatalf("case %d Rewind(%d): 帧早于目标", caseIndex, target)
					}
				}
			}
		}

		// 查询时刻覆盖：窗口内、边界点、窗口外（越界）、晚于最新帧。
		query(oldest)
		query(oldest - 1)
		query(newest)
		query(newest + DelayMs + 1)
		for q := 0; q < 6; q++ {
			query(oldest - 500 + rng.Int63n(newest-oldest+1500))
		}
	}
}
