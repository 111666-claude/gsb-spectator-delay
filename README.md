# spectator-delay

观战缓冲：固定容量的时间窗口缓冲、按时间取帧与回拖，只用 Go 标准库。

```
go test ./...
go run ./cmd/spectator-delay --sample view
go run ./cmd/spectator-delay --sample retain
go run ./cmd/spectator-delay --sample rewind
go run ./cmd/spectator-delay --sample growth
```

## 口径（README 为准）

- **缓冲按时间窗口裁**：只保留 `AtMs >= 最新帧.AtMs - (DelayMs + RetainMs)` 的帧，
  并且帧数不超过 `Capacity`；`Buffered()` 必须反映这两条约束。
- **取帧含边界**：`View(now)` 取 `AtMs <= now - DelayMs` 的最新一帧。
- **回拖含边界、越界要报错**：`Rewind(target)` 返回所有 `AtMs >= target` 的帧（升序）；
  `target` 早于缓冲里最旧的一帧时返回 `ErrOutOfRange`。
- **查询代价**：`Scanned()` 统计每次查询碰过的帧数，必须不超过 `ceil(log2(Buffered)) + 命中帧数`，
  也就是要按时间二分定位，不许每次从头全扫。
- **帧率无关**：同一 `DelayMs` 下 60fps 与 30fps 取到的时间点相差不超过一帧。
- **规模**：单会话 17.3 万帧（20 分钟 144fps）、每秒 1 万次查询，
  内存只跟 `Capacity` 有关。

## 输出契约（不改格式）

```
atms=800 ok=true
buffered=200 frames=125 scanned<=133
frames=13 first=800
buffered<=512
```

## 常量

```
DelayMs  = 200
RetainMs = 3000
Capacity = 512
```

## 目录

```
buffer.go               观战缓冲
cmd/spectator-delay     命令行入口
buffer_test.go          go test 用例
```
