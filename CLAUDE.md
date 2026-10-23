# ww-excel

macOS 試算表 App，目標是提供接近 MS Excel 的操作體驗。
技術棧：**Go + Wails3 + Svelte 5 + TypeScript (Vite)**。

完整規劃筆記：Obsidian `Mobile_Notes/ww_apps/ww_excel_plan.md`

## 目前進度

- 目前階段：**v0.3 完成**（2026-10-23），下一步是 v0.4（格式與 xlsx）
- 完成一個版本後，更新此區塊並勾選下方對應項目
- 已鎖定版本：Wails `v3.0.0-beta.34`（Go module 與 `@wailsio/runtime` 必須一致）、Svelte 5、Vite 8、vitest 5

## 常用指令

```bash
export PATH="$HOME/go/bin:$PATH"   # wails3 CLI is installed here; build tasks need it on PATH

wails3 dev                          # dev mode with hot reload
wails3 build                        # production build -> bin/ww-excel
wails3 generate bindings -clean=true -ts -i   # regenerate frontend/bindings after changing bound Go APIs

go test ./internal/...              # Go unit tests
go test -run x -bench . -benchmem ./internal/...
cd frontend && npm test             # vitest
cd frontend && npm run check        # svelte-check (needs up-to-date bindings)

# Perf harness: opens a window, runs, writes docs/perf-latest.md + docs/snapshot-*.png, quits
WW_BENCH=1 WW_BENCH_OUT="$PWD/docs/perf-latest.md" ./bin/ww-excel

# Editing self-test: drives the real controller with simulated keys, writes
# docs/selftest-latest.md + docs/selftest.png, quits (60s watchdog)
WW_SELFTEST=1 WW_BENCH_OUT="$PWD/docs/selftest-latest.md" ./bin/ww-excel

./bin/ww-excel path/to/file.csv     # open a .csv / .wwx on launch
```

- 要確認畫面時，跑上面的量測模式，再讀 `docs/snapshot-origin.png` 和 `docs/snapshot-end.png`，不需要螢幕錄製權限
- 量測結論整理在 `docs/perf.md`；每完成一版就補一節
- 一般啟動是空白文件；`WW_FAKE_ROWS` / `WW_FAKE_COLS` 可以載入假資料（量測模式會自動載入）
- 新增鍵盤或編輯行為時，在 `frontend/src/bench/selfTest.ts` 加上對應檢查項
- 新增或修改函數時，在 `testdata/golden/functions.tsv` 加上案例（`go test ./internal/formula -run TestGolden`）
- self-test 模擬不了原生 IME 和選單快捷鍵，這兩項每版都要手動確認

---

## 架構原則（必須遵守）

```
Svelte (WKWebView)                         Go Backend
├─ UI Chrome (Ribbon/FormulaBar/Tabs)      ├─ Workbook Model（稀疏 block 儲存、style table）
├─ Grid Renderer（Canvas 虛擬化）  ──cmd──▶ ├─ Command Bus + Undo/Redo（transaction）
├─ Viewport Cache（block 快取）    ◀─diff── ├─ Formula Engine（parser→AST→依賴圖→重算）
└─ Input Layer（選取/鍵盤/IME）            ├─ IO：xlsx（excelize）/ csv / 自有格式
                                           └─ macOS 整合：選單、多視窗、對話框
```

1. **Go 是唯一資料來源**。前端不持有完整資料，只有 viewport 附近的 block 快取。
2. **前端 → Go 只送 command**（`SetCellValue`、`ApplyStyle`、`InsertRows`…）；**Go → 前端用 event 推送變動 cells 的 diff**。
3. **禁止逐格呼叫 binding**。資料以 block 為單位（例如 64×32）批次讀取，並預取鄰近區塊。
4. **格線一律用 Canvas 繪製，不用 DOM table**。只有編輯中的儲存格疊一個 `<textarea>`（同時處理 IME 組字）。
5. 選取框、複製虛線框放在獨立的 overlay canvas；凍結窗格分成 4 個繪製區域。
6. 列高、欄寬用 Fenwick tree 或前綴和做「像素 ↔ 索引」換算，隱藏列欄的尺寸為 0。
7. **公式引擎自己寫**。excelize 只用於 xlsx IO，不使用它的 `CalcCellValue`。不引入 HyperFormula（GPL）。
8. 所有修改都要經過 Command（含 `Do` / `Undo`）。多步操作包成一個 transaction，對應一次 undo。
9. `Cell` 只存值、公式 ID、樣式 ID；樣式放在去重的 style table。
10. `internal/service/` 只做 Wails 綁定的薄層，邏輯全部放在 `internal/` 的純 Go 套件，**不啟動 Wails 也能測試**。
11. Wails3 API 可能變動：鎖定版本，前端只透過 `frontend/src/bridge/` 存取 binding 與 event。

## 專案結構

```
ww-excel/
├── main.go                    # Wails app entry, menus, windows
├── internal/
│   ├── model/                 # Workbook, Sheet, Cell, StyleTable, block storage
│   ├── command/               # Command interface, undo/redo, transactions
│   ├── formula/
│   │   ├── lexer/ parser/ ast/
│   │   ├── eval/              # evaluator, value types, coercion
│   │   ├── depgraph/          # dependency graph, recalc scheduler
│   │   └── functions/         # math.go, lookup.go, text.go, date.go ...
│   ├── numfmt/                # Excel number format parser/renderer
│   ├── io/                    # xlsx (excelize), csv, native format
│   └── service/               # Wails-bound services (thin layer)
├── frontend/src/
│   ├── grid/                  # canvas renderer, layout (fenwick), hit-testing
│   ├── input/                 # keyboard map, selection, editor overlay, IME
│   ├── stores/                # Svelte stores: selection, viewport cache
│   ├── components/            # Ribbon, FormulaBar, SheetTabs, dialogs
│   └── bridge/                # typed wrappers over generated bindings/events
└── testdata/golden/           # Excel-generated formula expectations
```

## 公式引擎規格

- Parser：Pratt parser，支援 `A1`、`$A$1`、`A1:B5`、`Sheet2!A1`、`'My Sheet'!A1`、`A:A`、`1:1`、具名範圍、陣列常數 `{1,2;3,4}`
- AST 在 parse 後快取，重算時只做 evaluate
- 依賴圖：cell → dependents；範圍依賴用 interval 結構，不展開成每一格
- 重算：dirty 標記加拓撲排序；要偵測循環參照；volatile 函數（`NOW`、`RAND`、`OFFSET`、`INDIRECT`）每次都重算
- 值型別：Number / String / Bool / Error / Range / Array；隱式轉型遵循 Excel 規則
- 日期採 1900 序列值系統，需重現 Excel 的 1900 閏年 bug
- **插入或刪除列欄時要調整參照，參照失效時產生 `#REF!`**，AST 設計要提前支援
- 函數行為以真實 Excel 為準，加入 `testdata/golden/` 做 golden test

## 開發規範

- 回覆與 commit message 使用繁體中文，程式碼註解使用英文
- Go：`gofmt`；在 `internal/` 撰寫 table-driven test；效能敏感路徑（重算、block 讀取、渲染資料準備）附 benchmark
- 前端：Svelte 5 runes 加 TypeScript strict
- 快捷鍵以 macOS 為準（⌘ 取代 Ctrl），行為對齊 Mac 版 Excel
- 改動 IME 或編輯器相關程式碼時，要以注音和倉頡實測

## 效能目標

- 100 萬列快速捲動維持 60fps
- 1 萬個公式的連鎖重算少於 100ms
- 10 萬個有值的儲存格，記憶體用量合理（每版量測並記錄）

---

## 分階段執行計畫

每一版都要能用、能 demo，前一版的架構不需推翻。依序執行，不跳版。

### v0.1 — 技術驗證
- [x] Wails3 + Svelte 5 + TS 專案骨架
- [x] Canvas 虛擬化格線：欄列標題、捲動
- [x] Go 端產生假資料，前端以 block 讀取
- [x] 實測 IPC 延遲（單次呼叫、批次 block、event 推送），結果記錄在 `docs/perf.md`
- [x] 建立 `bridge/` 包裝層
- **驗收**：100 萬列捲動 60fps；10 萬格資料時記憶體合理 → ✅ 達成（見 `docs/perf.md`）
- 留到後續版本：列標題依位數自動加寬（目前固定 60px）、文字溢出到相鄰空格（目前會被裁切）、稀疏 block 節省記憶體
- v0.2 要注意：diff event 每次編輯合併成一個事件，連續推送時延遲會累積 → 已實作

### v0.2 — 可編輯的試算表
- [x] 選取模型（active cell、range、多重範圍）
- [x] 鍵盤導航：方向鍵、⌘+方向鍵、Shift、Tab/Enter、Home/End、PgUp/PgDn
- [x] textarea 疊層編輯器（直接輸入 / F2 / Esc / Enter / ⌥+Enter）
- [x] IME 中文輸入（常駐 textarea 設計；需手動實測注音、倉頡）
- [x] Go Workbook Model（稀疏 block 儲存）
- [x] Command Bus + Undo/Redo（⌘Z / ⇧⌘Z）
- [x] 開檔存檔：CSV 與自有格式（`.wwx` = gzip JSON）
- [x] 原生選單列、檔案對話框
- **驗收**：能輸入資料，Undo/Redo 正常 → ✅ self-test 30/30；中文輸入 → 待手動確認
- 設計重點：
  - 編輯器是一個常駐、隨時有焦點的 `<textarea>`，閒置時透明地放在作用中儲存格上，所以 IME 的第一個按鍵就會進入組字，候選字視窗也會出現在儲存格旁
  - 所有後端呼叫依序排隊（`Sheet.svelte` 的 `enqueue`）；選單和 ⌘ 快捷鍵由 `runMenu` 處理，**不能在佇列內呼叫**，否則會自己等自己而卡死
  - ⌘ 快捷鍵可能同時收到 keydown 和選單事件，`runMenu` 會忽略 150ms 內的重複動作
  - GetBlocks 回傳文件版本，前端丟棄比最新 diff 舊的結果；開新文件時用 generation 計數擋掉舊請求
  - 一次變動超過 5,000 格時只送「失效」通知，前端清掉快取重抓
  - ⌘Q 不經過視窗的 closing hook，所以用 `ShouldQuit` 把它轉到關閉視窗的未儲存確認
- 留到後續版本：⌥Enter 後自動換行與列高（v0.4 格式）、⌘A 先選目前區域再選整張（v0.5）、列標題自動加寬、Big5 編碼的 CSV

### v0.3 — 公式引擎（核心）
- [x] Lexer / Pratt Parser / AST
- [x] 依賴圖、增量重算、循環參照偵測
- [x] 值型別系統、錯誤值、日期序列值
- [x] 約 90 個函數（數學統計、邏輯、查閱、文字、日期、資訊），清單見 `internal/formula/functions/`
- [x] 公式列（可編輯）、參照彩色框、point mode、F4 / ⌘T 切換 `$`
- [x] 函數自動完成與參數提示
- [x] Golden test 測試集（`testdata/golden/functions.tsv`，211 項）
- **驗收**：golden test 全數通過 ✅；1 萬個公式的連鎖重算 6.4ms ✅（見 `docs/perf.md`）
- 設計重點：
  - 套件：`formula/lexer` → `parser`（Pratt）→ `ast`（含 `Format` 標準化輸出、`Adjust` 插入／刪除列欄、`Translate` 複製公式）→ `eval`（值、轉型、運算子、條件）→ `functions`（`init` 註冊）→ `depgraph` → `formula`（引擎）
  - `model.Cell.F *Formula` 存公式文字和編譯結果，值欄位存計算結果；Undo 直接還原整個 Cell，之後重算
  - 重算：從變動的儲存格 BFS 標記 dirty，再用 Kahn 拓撲排序依序計算，**不用遞迴**；排不完的節點用 Tarjan SCC 找出循環，照 Excel 的做法設為 0，並持續回報到狀態列
  - 範圍依賴依 block 分桶；涵蓋超過 64 個 block 的範圍（`A:A`）另存清單掃描
  - 公式在輸入時就編譯；語法錯誤或引數數量不對會被拒絕，前端回到編輯狀態並顯示原因。未知函數照 Excel 允許輸入，結果是 `#NAME?`
  - 沒有動態陣列：範圍在運算中取隱含交集，陣列結果只顯示左上角；`SUMPRODUCT` 這類函數的參數用陣列模式計算
  - 數字顯示、比較和 `ROUND` 都先四捨五入到 15 位有效數字（`=0.1+0.2=0.3` 是 TRUE、`ROUND(2.675,2)` 是 2.68）
- **Golden 期望值目前是依 Excel 文件手寫的**，還沒用真實 Excel 驗證。v0.4 有 xlsx 讀檔後，改成讀 Excel 存檔裡的快取值做比對
- 留到後續版本：公式文字的參照上色（textarea 無法上色）、拖曳參照框調整範圍、`INDEX` 回傳參照（`A1:INDEX(...)`）、空格交集運算子、動態陣列與溢出、具名範圍（v0.6）、日期字串自動轉換（`="2026/1/1"+1`）

### v0.4 — 格式與 xlsx
- [ ] 樣式系統（字型、填滿、框線、對齊、換行）與 style table
- [ ] 數值格式解析器（`internal/numfmt`）
- [ ] 調整列高欄寬、雙擊自動欄寬
- [ ] 合併儲存格
- [ ] 多工作表（新增 / 刪除 / 改名 / 排序）
- [ ] xlsx 匯入匯出（excelize），不支援的內容盡量原樣保留
- **驗收**：一般報表 xlsx 外觀大致正確，存回後 Excel 能正常開啟

### v0.5 — 生產力操作
- [ ] 剪貼簿與 Excel 互通（TSV 與 HTML）
- [ ] 選擇性貼上（值、格式、轉置）
- [ ] 填滿控點、自動數列、日期與自訂清單
- [ ] 插入 / 刪除列欄，公式參照自動調整
- [ ] 凍結窗格、隱藏列欄
- [ ] 尋找與取代、排序、自動篩選
- [ ] 多視窗（一份文件一個視窗）、最近檔案
- **驗收**：能取代 Excel 處理日常輕量工作

### v0.6 — 進階資料功能
- [ ] 條件式格式、資料驗證（下拉選單）
- [ ] 具名範圍、跨工作表參照
- [ ] 函數擴充到 200–300 個
- [ ] 註解、表格（Table）
- **驗收**：常見財務和統計模板能正確計算

### v0.7 — 視覺化與輸出
- [ ] 圖表（長條、折線、圓餅、散佈），與資料連動
- [ ] 浮動物件層（圖片、圖表的拖曳與縮放）
- [ ] 列印預覽、匯出 PDF、版面設定

### v1.0 — 正式版打磨
- [ ] 效能調校（大檔開啟、記憶體、重算並行化）
- [ ] 自動儲存、當機復原
- [ ] 偏好設定、深色模式、繁中 / 英文介面
- [ ] 程式碼簽章、notarization、DMG 打包、自動更新
- [ ] VoiceOver 基本支援

### Backlog（v1.x 之後）
動態陣列與溢出（`FILTER`、`SORT`、`UNIQUE`）、`LET` / `LAMBDA`、樞紐分析表、巨集或腳本、外掛系統、多人協作（CRDT）

---

## 風險提醒

1. Wails3 可能仍是 alpha 或 beta：鎖定版本，binding 只透過 `bridge/` 使用
2. IME 與 Canvas 的配合：從 v0.2 開始持續實測
3. 插入 / 刪除列欄時的參照調整：v0.3 設計 AST 時就要納入
4. xlsx 相容性：讀不懂的內容要保留，不能破壞檔案
5. 數值格式與日期：錯誤很明顯，必須用測試把關
