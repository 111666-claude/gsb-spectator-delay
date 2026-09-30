package spectator

import "testing"

func TestPushKeepsShortRecording(t *testing.T) {
	buffer := NewBuffer()
	for index := 0; index < 5; index++ {
		buffer.Push(Frame{Seq: index, AtMs: int64(index * 16)})
	}
	if buffer.Buffered() != 5 {
		t.Fatalf("短录像不该被裁：%d", buffer.Buffered())
	}
}

func TestViewOnEmptyBuffer(t *testing.T) {
	if _, ok := NewBuffer().View(1000); ok {
		t.Fatal("空缓冲不该给出画面")
	}
}

func TestRewindReturnsLaterFrames(t *testing.T) {
	buffer := NewBuffer()
	for index := 0; index < 20; index++ {
		buffer.Push(Frame{Seq: index, AtMs: int64(index * 16)})
	}
	frames, err := buffer.Rewind(100)
	if err != nil {
		t.Fatalf("回拖不该报错：%v", err)
	}
	if len(frames) == 0 {
		t.Fatal("回拖应该给出至少一帧")
	}
	for _, frame := range frames {
		if frame.AtMs < 100 {
			t.Fatalf("回拖给出的帧比目标时刻还早：%+v", frame)
		}
	}
}

func TestConstants(t *testing.T) {
	if DelayMs != 200 || RetainMs != 3000 || Capacity != 512 {
		t.Fatal("观战缓冲常量被改了")
	}
}
