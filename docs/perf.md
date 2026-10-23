# 效能量測紀錄

## 如何量測

```bash
export PATH="$HOME/go/bin:$PATH"
wails3 build
WW_BENCH=1 WW_BENCH_OUT="$PWD/docs/perf-latest.md" ./bin/ww-excel
```

量測模式會開啟視窗，自動跑完所有項目，把報告寫到 `docs/perf-latest.md`，格線截圖寫到 `docs/snapshot-*.png`，然後自行關閉。
一般模式下按狀態列的「效能量測」按鈕也能跑同一套量測，結果顯示在視窗內。
Go 端 benchmark：`go test -run x -bench . -benchmem ./internal/...`

---

## v0.1（2026-10-23）

環境：Apple Silicon、macOS 27、WKWebView、1280×800 @2x、資料 20,000 列 × 5 欄（約 10 萬格）加上邊界標記。

### 結論

| 項目 | 目標 | 結果 | 判定 |
|---|---|---|---|
| 100 萬列捲動 | 60fps | 三種捲動情境都是 60fps，300 frames 內掉幀 0 | ✅ |
| Grid 繪製成本 | < 8ms / frame | p50 1–2ms，p95 3ms 以內 | ✅ |
| 空呼叫延遲 | — | 平均 0.21ms | ✅ |
| 一個 viewport 的資料（9 blocks） | < 16ms | 平均 1.2ms，94KB JSON | ✅ |
| 捲動時資料送達 | 1–2 frame 內 | 平均 2.25ms，p99 5ms | ✅ |
| 10 萬格的記憶體 | 合理 | Go heap 23.6MB | ✅（可再優化） |

### 觀察與後續

- **呼叫延遲不是瓶頸**：一次 binding 呼叫的成本主要在 JSON 序列化，每 KB 約 0.01ms。block 批次讀取的設計可以沿用。
- **Event 推送**：小事件約每秒 6 萬個；帶 1 個 block（10KB）的事件約每秒 4,600 個。連續推送時延遲會累積（p95 約 40ms），所以 v0.2 的 diff 推送要**合併成每次編輯一個事件**，不要逐格發。
- **Go 記憶體**：每個 block 是 64×32 格的固定陣列，每格 32 bytes，所以一個 block 佔 64KB。資料只有 5 欄時大部分空間沒用到。v0.4 導入 style table 或共用字串時，可以改成「稀疏 block」或縮小 `Cell`。
- **計時精度**：WKWebView 的 `performance.now()` 只到 1ms，所以表中 p50 = 0/1 是精度限制，判讀以 mean 為準。
- **捲動映射**：預設列高 20px 時，100 萬列總高約 2,100 萬 px，低於 WebKit 上限，因此捲動是 1:1。列高總和超過 3,000 萬 px 時才會按比例縮放。

### 原始報告

#### v0.1 Perf Report

- 時間：2026-10-23T12:55:06.120Z
- 資料：1,048,576 used rows × 16,384 used cols（含邊界標記）
- 視窗：1280×800 @2x
- UA：Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) wails.io
- 注意：WKWebView 的 `performance.now()` 精度為 1ms，低於 1ms 的數值會顯示為 0 或 1，請以 mean 判讀。

#### Binding 呼叫延遲（空呼叫 Ping）

| case | n | mean ms | p50 ms | p95 ms | p99 ms | total ms | per call ms |
| --- | --- | --- | --- | --- | --- | --- | --- |
| sequential | 500 | 0.21 | 0.00 | 1.00 | 1.00 |  |  |
| 100 parallel | 100 |  |  |  |  | 5.00 | 0.05 |

#### GetBlocks 批次讀取

| case | n | mean ms | p50 ms | p95 ms | p99 ms | cells | payload KB |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 blocks | 40 | 0.33 | 0.00 | 1.00 | 1.00 | 320 | 10.5 |
| 9 blocks | 40 | 1.18 | 1.00 | 2.00 | 2.00 | 2880 | 94.4 |
| 25 blocks | 40 | 2.80 | 3.00 | 3.00 | 4.00 | 8000 | 262.2 |
| 64 blocks | 40 | 6.85 | 7.00 | 8.00 | 8.00 | 20480 | 671.2 |

> 一個 viewport 約 3×3 blocks（含預取環則約 5×5）。

#### Event 推送（Go → 前端）

| case | received | total ms | events/s | lag p50 ms | lag p95 ms |
| --- | --- | --- | --- | --- | --- |
| 200 events × 0 blocks | 200 | 3.00 | 66667 | 1.00 | 3.00 |
| 200 events × 1 blocks | 200 | 43.00 | 4651 | 23.00 | 41.00 |
| 50 events × 9 blocks | 50 | 25.00 | 2000 | 12.00 | 23.00 |

> lag 以 Go 端 UnixMilli 與前端 Date.now() 相減，精度為 1ms。

#### Canvas 捲動

| case | frames | fps | frame p95 ms | frame max ms | dropped (>20.7ms) | draw p50 ms | draw p95 ms |
| --- | --- | --- | --- | --- | --- | --- | --- |
| steady 200px/frame | 300 | 60.0 | 18.00 | 19.00 | 0 | 2.00 | 3.00 |
| random jumps over 1M rows | 300 | 60.0 | 18.00 | 18.00 | 0 | 1.00 | 2.00 |
| diagonal | 300 | 60.0 | 18.00 | 19.00 | 0 | 1.00 | 1.00 |

> 每個 frame 以程式設定捲動位置並同步重繪；draw 為 grid layer 單次繪製耗時。

#### 捲動期間的 block 載入（資料送達畫面的延遲）

| case | n | mean ms | p50 ms | p95 ms | p99 ms | blocks/call mean | blocks/call max |
| --- | --- | --- | --- | --- | --- | --- | --- |
| GetBlocks during scroll | 199 | 2.25 | 2.00 | 4.00 | 5.00 | 6.4 | 8 |

> 從 ensure() 發出請求到資料寫入快取的時間；載入後下一個 frame 即重繪。

#### 記憶體

| metric | value |
| --- | --- |
| Go heap alloc | 23.3 MB |
| Go sys | 67.1 MB |
| Frontend cached blocks | 1500 |

> WKWebView 不提供 performance.memory，前端記憶體請用「活動監視器」觀察 WebContent 程序。

---

## v0.2（2026-10-23）

加入編輯、選取與 Undo/Redo 後重跑同一套量測，數字與 v0.1 一致：

| 項目 | v0.1 | v0.2 |
|---|---|---|
| 捲動 fps（三種情境） | 60 / 60 / 60 | 60 / 60 / 60 |
| Grid 繪製 p95 | 3ms | 3ms |
| 空呼叫平均 | 0.21ms | 0.21ms |
| 9 blocks 平均 | 1.18ms | 1.18ms |
| 捲動時資料送達平均 | 2.25ms | 2.38ms |
| Go heap（10 萬格） | 23.6MB | 23.5MB |

- 選取層改成多範圍繪製（含整欄／整列判斷），沒有增加可量測的繪製成本
- 編輯 self-test（`WW_SELFTEST=1`）30/30 通過，結果見 `docs/selftest-latest.md`（不進版控）

---

## v0.3（2026-10-23）

加入公式引擎後重跑同一套量測，並新增「公式重算」項目。

| 項目 | v0.2 | v0.3 |
|---|---|---|
| 捲動 fps（三種情境） | 60 / 60 / 60 | 60 / 60 / 60 |
| Grid 繪製 p95 | 3ms | 3ms |
| 9 blocks 平均 | 1.18ms | 1.27ms |
| 64 blocks 平均 | 6.85ms | 7.58ms |
| Go heap（10 萬格） | 23.5MB | 28.2MB |
| **1 萬個公式連鎖重算** | — | **6.4ms**（驗收 < 100ms ✅） |
| 10 萬個公式連鎖重算 | — | 87ms |

Go benchmark（`go test -bench . ./internal/formula`）：

| benchmark | 結果 |
|---|---|
| 1 萬個公式連鎖重算 | 5.8ms / 次 |
| 1 萬個 `SUM(10 格)`，改其中一格 | 24µs / 次（只重算受影響的 10 個公式） |
| 20 萬層連鎖（測 stack） | 通過；重算不使用遞迴 |

- **記憶體**：`model.Cell` 多了公式指標（32 → 40 bytes），heap 增加約 20%。v0.4 改成稀疏 block 時一併處理。
- **GetBlocks 慢了約 10%**：數字改用 Excel 的一般格式（15 位有效數字），每格多一次轉換。目前仍遠低於 16ms 的 frame 預算；之後可以快取顯示字串。
- Go sys 536MB 是量測本身建出 10 萬個公式的暫時工作簿造成的，不是 app 平常的用量。
- 編輯 self-test 49/49 通過（v0.2 的 30 項加上公式相關 19 項）。
