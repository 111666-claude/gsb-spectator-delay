# spectator-delay

观战延迟缓冲：按时间对齐画面、回拖取帧、窗口裁剪，只用 Go 标准库。

```
go test ./...
go run ./cmd/spectator-delay --sample view
go run ./cmd/spectator-delay --sample retain
go run ./cmd/spectator-delay --sample rewind
```

## 口径（README 为准）

- **取帧含边界**：`View(nowMs)` 取 `AtMs <= nowMs - DelayMs` 的最新一帧。
  `--sample view`（60fps、每 16ms 一帧、停在 1000ms）按口径是 `atms=800`，
  现在是 `atms=784`。
- **缓冲按时间裁**：只保留 `AtMs >= 最新帧.AtMs - (DelayMs + RetainMs)` 的帧，
  帧率与缓冲条数都不影响能回拖多久。`--sample retain`（10 秒 60fps 画面回拖到 8000ms）
  按口径是 `buffered=200 frames=125`（保留窗口最早 6800ms），现在是
  `buffered=30 frames=30`——缓冲只剩最后 0.46 秒，回拖范围被帧率吃掉了。
- **回拖含边界、越界要报错**：`Rewind(targetMs)` 返回所有 `AtMs >= targetMs` 的帧（升序），
  `targetMs` 早于缓冲里最旧的一帧时返回 `ErrOutOfRange`。
  `--sample rewind`（回拖到 800ms，正好落在一帧上）按口径 `frames=13 first=800`，
  现在是 `frames=12`。
- 不变量：同一 delayMs 下 60fps 与 30fps 取到的时间点相差不超过一帧；
  `Rewind` 给出的帧按时间升序且都 `>= targetMs`；缓冲里不会有比最旧帧更早的帧。
- 规模：单次 `Push`/`View`/`Rewind` 摊还 O(1) 或 O(命中帧数)，内存只跟保留窗口的时长有关；
  不许把整场录像留下再按时间过滤。

## 现在的行为

- `Push` 只按 `MaxFrames` 条数裁剪，30 帧以外直接丢掉，帧率越高能回拖的时间越短。
- `View` 用 `AtMs < limit`，边界那一帧永远取不到。
- `Rewind` 用 `AtMs > targetMs`，并从不返回 `ErrOutOfRange`。

## 输出

```
atms=800 ok=true
buffered=200 frames=125
frames=13 first=800
```

## 目录

```
buffer.go               观战缓冲
cmd/spectator-delay     命令行入口
buffer_test.go          go test 用例
```
