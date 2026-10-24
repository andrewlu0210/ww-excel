# ww-excel

macOS 試算表 App，目標是提供接近 MS Excel 的操作體驗。
技術棧：**Go + Wails3 + Svelte 5 + TypeScript (Vite)**。

完整規劃筆記：Obsidian `Mobile_Notes/ww_apps/ww_excel_plan.md`

## 目前進度

- 目前階段：**v0.5 完成**（2026-10-24），下一步是 v0.6（進階資料功能）
- 完成一個版本後，更新此區塊並勾選下方對應項目
- 已鎖定版本：Wails `v3.0.0-beta.34`（Go module 與 `@wailsio/runtime` 必須一致）、excelize `v2.11.0`、Svelte 5、Vite 8、vitest 5

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

./bin/ww-excel path/to/file.xlsx    # open a .xlsx / .csv / .wwx on launch

go run ./tools/goldenxlsx           # build testdata/golden/excel-check.xlsx (see testdata/golden/README.md)
```

- 要確認畫面時，跑上面的量測模式，再讀 `docs/snapshot-origin.png` 和 `docs/snapshot-end.png`，不需要螢幕錄製權限
- 量測結論整理在 `docs/perf.md`；每完成一版就補一節
- 一般啟動是空白文件；`WW_FAKE_ROWS` / `WW_FAKE_COLS` 可以載入假資料（量測模式會自動載入）
- 新增鍵盤或編輯行為時，在 `frontend/src/bench/selfTest.ts` 加上對應檢查項
- 新增或修改函數時，在 `testdata/golden/functions.tsv` 加上案例（`go test ./internal/formula -run TestGolden`）
- self-test 模擬不了原生 IME 和選單快捷鍵，這兩項每版都要手動確認
- 跑 self-test 或量測前先確認沒有殘留的 `bin/ww-excel` 程序（`pgrep -fl bin/ww-excel`）；Go 端有看門狗會在 3 分鐘後強制結束

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
- [x] 樣式系統（字型、填滿、框線、對齊、換行）與 style table
- [x] 數值格式解析器（`internal/numfmt`）：色彩區段、民國年、上午/下午、內建格式 ID、輸入自動判斷格式
- [x] 調整列高欄寬（拖曳標題邊界）、雙擊自動調整
- [x] 合併儲存格（跨欄置中、合併感知的選取與移動）
- [x] 多工作表（新增 / 刪除 / 改名 / 排序，公式參照跟著更新）
- [x] xlsx 匯入匯出（excelize），原地存檔保留不支援的內容
- [x] UI：常用工具列、顏色選擇器、⌘1 數值格式對話框、工作表分頁
- **驗收**：self-test 75/75（含 xlsx 存讀）✅；效能維持 60fps ✅（見 `docs/perf.md`）；**用真實 Excel 開啟存出的檔案 → 待手動確認**
- 設計重點：
  - `model.Cell.S` 是樣式 ID，`StyleTable` 去重且 ID 永不重用（Undo 安全）；`Layout` 存列高欄寬、整列整欄樣式、合併
  - 整欄 / 整列格式寫進 `Layout.ColStyles/RowStyles`，不建立儲存格；有效樣式 = 儲存格 → 列 → 欄
  - Delete 只清內容、保留格式；輸入時保留格式，格式為「通用」時才套用自動判斷的格式；公式結果沿用第一個參照的格式（`=A1+7` 仍是日期）
  - **xlsx 原地存檔**：保留開檔時的 excelize 檔案，只寫入變動的儲存格與版面（`Document.changedCells/layoutDirty`），圖表、註解、資料驗證等不會遺失；工作表先改成暫存名再改名，處理名稱互換
  - **不要用 excelize 的 `GetCellStyle` 讀檔**：它會在記憶體中補滿空儲存格，讓原地存檔膨脹；樣式和公式位置由 `rawcells.go` 直接串流解析 XML 取得
  - 較新函數寫檔時加 `_xlfn.` 前綴（`ast.WithFilePrefixes`），讀檔時去掉
  - 貨幣（ID 5–8）與地區相關的內建格式一律寫成自訂格式碼，excelize 會丟掉它們
  - 我們看不懂的公式（例如結構化參照）保留 Excel 的快取值，不重算
  - 前端：`styleStore` 隨 GetBlocks / Layout / diff 同步樣式；`Grid` 套用版面時要 `untrack`，否則會觸發無限 effect；元件卸載後的回呼（區塊抓取、公式列更新）必須忽略
  - 工作表結構變動時 App 依名稱找回目前工作表並重新掛載 Sheet（快取以工作表索引為 key）
- 已知限制：存出的 xlsx 不含文字 / 錯誤 / 布林公式結果的快取值（Excel 開啟會重算，Quick Look 會看到空白）；xlsx 開檔 10 萬格約 1 秒；不支援的格式屬性（文字旋轉、圖樣填滿等）在**被編輯過的儲存格**上會遺失
- 留到後續版本：Golden 期望值用真實 Excel 驗證（工具已備好，見 `testdata/golden/README.md`）、編輯中的部分文字格式（rich text）、條件式格式（v0.6）、凍結窗格與隱藏列欄 UI（v0.5）、稀疏 block 節省記憶體

### v0.5 — 生產力操作
- [x] 剪貼簿與 Excel 互通（TSV 與 HTML）
- [x] 選擇性貼上（值、格式、轉置）
- [x] 填滿控點、自動數列、日期與自訂清單
- [x] 插入 / 刪除列欄，公式參照自動調整
- [x] 凍結窗格、隱藏列欄
- [x] 尋找與取代、排序、自動篩選
- [x] 多視窗（一份文件一個視窗）、最近檔案
- **驗收**：能取代 Excel 處理日常輕量工作 → ✅ self-test 113/113；與 Excel 互相複製貼上、多視窗、Finder 開檔 → 待手動確認
- 設計重點：
  - 剪貼簿：⌘C 時 Go 存一份內部 clip（含公式 AST 與樣式值，不依賴來源活頁簿），並把 TSV 寫進系統剪貼簿；前端在使用者手勢內再嘗試寫入 HTML 表格。貼上時剪貼簿文字和上次複製的相同就用內部 clip（公式平移、格式、合併），否則當成輸入逐格解析
  - ⌘V 閒置時由 Go 讀系統剪貼簿；選單的原生「貼上」會觸發 textarea 的 paste 事件，兩條路都經過 `runMenu` 去重。編輯中 ⌘C/⌘X/⌘V 交給 textarea 原生處理
  - 剪下貼上是「移動」：整本活頁簿參照到來源範圍的公式改指向新位置（`ast.Move`）；剪下後有任何編輯就取消
  - 插入／刪除列欄：`model.Sheet.Shift` 搬儲存格與版面，`ast.Adjust` 調整所有公式，Undo 用公式快照完整還原（含 `#REF!`）。xlsx 原地存檔時用 excelize 重放 InsertRows/RemoveRow，並手動搬註解
  - 凍結窗格：繪製分成最多 4 個區域，每區有自己的捲動量和裁切框；凍結以絕對列欄為準，凍結時捲回左上角
  - 篩選：`Layout.Filter` 記錄範圍和每欄允許的顯示值，用隱藏列實作；xlsx 讀寫 `_xlnm._FilterDatabase` 與 autoFilter 範圍
  - 多視窗：每個視窗一個 `Session`，`Hub` 依 binding 呼叫的視窗（`application.WindowKey`）找到 session；Go 用 `window.EmitEvent` 推送，前端 bridge 依 sender 過濾。開檔時如果目前視窗是沒動過的新文件就直接取代，否則開新視窗；已開啟的檔案會切到該視窗
  - 最近檔案存在 `~/Library/Application Support/ww-excel/recent.json`；Info.plist 註冊 xlsx/csv（Alternate）與 wwx（Owner）
- self-test 會寫入真實的系統剪貼簿，也不測多視窗（新視窗會再跑一次 self-test）
- 留到後續版本：插入／刪除儲存格並右移或下移（目前只支援整列整欄）、轉置貼上時參照不轉置、移除篩選無法寫回已存在的 xlsx（excelize 沒有刪除 API）、凍結時保留目前捲動位置、從 Excel 貼上 HTML 格式、拖曳選取範圍搬移、⌥ 拖曳填滿控點

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
