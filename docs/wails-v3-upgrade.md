# Wails v3.0.0 升級紀錄

日期：2026-11-13。來源版本：`v3.0.0-beta.34`；目標版本：`v3.0.0`。

## 變更範圍

- Go module 與 npm runtime 固定為 `3.0.0`，同步更新 `go.sum` 與 `package-lock.json`。
- 本機 `/Users/andrew/go/bin/wails3` 更新為 `v3.0.0`，使用 Go 1.27.3 建置。
- 新版 CLI 重新產生 4 個 services、114 個 methods、77 個 models、9 個 events；追蹤中的 bindings 沒有內容差異。
- 更新 README、專案筆記與 bridge 註解。
- 打包腳本改為優先使用呼叫端 PATH 中的 CLI，找不到時才使用 `$HOME/go/bin`，方便固定工具版本。
- 未改動 application、視窗、事件、選單、對話框或試算表業務邏輯。

直接比對兩版 Go module，`pkg/application` 的差異集中在 Linux child-frame guard；macOS 實作沒有差異。現有自訂 Taskfile 與打包流程可繼續使用。

工作區原有的四個圖示檔案及 `ww-excel.png` 保留原內容；本次建置前後 SHA-1 相同，未納入升級提交。

## 驗證結果

環境：macOS 27.2.1、Apple Silicon arm64、Go 1.27.3、Node/npm 與專案既有前端工具。

| 檢查 | beta.34 基準 | v3.0.0 |
| --- | --- | --- |
| `go test ./...` | 通過 | 通過 |
| `npm run check` | 0 errors、0 warnings | 0 errors、0 warnings |
| `npm test` | 31 個檔案、294 項通過 | 31 個檔案、294 項通過 |
| `npm run build` | 通過 | 通過 |
| production Go executable | 建置成功 | `wails3 build` 成功 |
| `wails3 dev` | 未另測 | 開發 App 啟動並成功連上 Vite 9245 |
| `scripts/package.sh` | 未另測 | App／DMG 建置成功，ad-hoc 簽章驗證通過 |
| 真實桌面 `WW_SELFTEST` | 既有 JS 錯誤，未完成 | 相同 JS 錯誤，未完成 |
| 打包後 App 的 `WW_BENCH` | 未另測 | 完成並產生報告 |

### 既有 self-test 問題

兩版 production self-test 的錯誤報告完全相同：

```text
TypeError: undefined is not an object (evaluating 'h.click')
```

開發模式未壓縮的堆疊將問題定位至 `frontend/src/bench/cellFormatDialogSelfTest.ts` 的 `fillButton.click()`。該測試以 button 的 `textContent` 搜尋「填滿色彩」，但目前 `NumberFormatDialog.svelte` 將文字放在 button 外的 span，button 透過 `aria-labelledby` 引用該標籤，因而找不到元素。

這是升級前已存在的測試選取器問題。本次保留原測試與 UI，沒有將桌面 self-test 列為通過；該錯誤也阻止後續格式對話框測試完成。需另行修正測試選取器並重新執行完整 self-test。

### 打包後 App 的執行驗證

透過既有 benchmark 確認 bindings、事件、區塊讀取、Canvas 與公式重算均可運作：

- Ping：完成 500 次循序呼叫與 100 次並行呼叫。
- Event：200／200、200／200、50／50，全數收到。
- GetBlocks：1、9、25、64 blocks 的批次讀取完成。
- Canvas：三種 300-frame 捲動情境均為 60 fps。
- 公式：1,000、10,000、100,000 個公式連鎖重算完成。

這些數值僅確認本次執行狀態；沒有相同條件下的 beta.34 benchmark，不據此宣稱效能改善。

### 訊息與限制

- Go linker 仍有既有的 macOS deployment target 警告；建置成功，但未在 macOS 12 實機驗證。
- npm 仍提示不支援 `.npmrc` 的 `minimum-release-age`；安裝結果另有 2 個 low severity vulnerabilities，未在此升級中擴大更新其他套件。
- Vite 仍提示單一 bundle 超過預設大小門檻。
- App 採本機 ad-hoc 簽章，未使用 Developer ID 簽章、未公證、未對外發布。
- 未驗證 Intel／Universal 產物、Windows、Linux、iOS 或 Android。

## 待人工驗收

本次環境未提供可操作原生 UI 的工具；以下項目仍需人工驗收，不包含在自動測試通過範圍內：

- [ ] 多文件事件隔離、切換視窗後的選單狀態。
- [ ] 未儲存文件的關閉與 ⌘Q：儲存、捨棄、取消，以及多份未儲存文件。
- [ ] 開檔、另存、圖片與 PDF 的原生對話框，包含取消操作。
- [ ] 原生選單快捷鍵、文字欄位與格線的剪貼簿操作、語系切換。
- [ ] Retina／外接螢幕的視窗位置還原。
- [ ] Finder 開檔、列印、外觀切換與當機後復原。

## 產物與重現

- App：`dist/WW Excel.app`。
- DMG：`dist/WW-Excel-1.3.3.dmg`；沿用專案版本 1.3.3，僅供本機驗收。
- 本次原始 logs、報告、截圖與 beta.34 基準執行檔：`/private/tmp/ww-excel-wails-v3.0.0/`（暫存資料，不保證永久保存）。
- 舊 CLI 備份：`/private/tmp/ww-excel-wails-v3.0.0/wails3-beta.34`。

```bash
export PATH="$HOME/go/bin:$PATH"
wails3 version
go test ./...
(cd frontend && npm run check && npm test)
wails3 build
WW_SELFTEST=1 WW_BENCH_OUT="$PWD/docs/selftest-latest.md" ./bin/ww-excel
scripts/package.sh
```

## 回復方式

還原升級提交中的依賴、lockfiles 與文件，重新安裝 CLI，再產生 bindings、建置前端與 App。不要只降 Go module 而保留新版 runtime 或 CLI。

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
export PATH="$HOME/go/bin:$PATH"
(cd frontend && npm ci)
wails3 generate bindings -clean=true -ts -i
wails3 build
```
