# 安全性檢查（2026-11-06）

範圍：目前工作目錄原始碼與鎖定相依版本，主要針對 macOS 正式版的檔案匯入、公式計算、WebView bindings、檔案讀寫與更新流程。這是程式碼審查、已知漏洞掃描及小型受控重現，不是完整滲透測試或安全認證。初次審查未修改功能程式碼及相依版本；後續第 13–16 項修補結果見文末。

## 優先處理

### 1. P1：畸形 XLSX 樞紐分析表可在開檔時觸發 panic（已修補）

- 位置：`internal/io/xlsxfile/pivots.go:23`，開檔會呼叫 Excelize `GetPivotTables`。
- 審查時版本：`v2.11.1-0.20260915055537-22f76f9acb94`。
- 官方公告：[GO-2026-6695 / GHSA-mx22-3794-2vpv](https://github.com/qax-os/excelize/security/advisories/GHSA-mx22-3794-2vpv)，為未檢查 pivot-cache 欄位索引造成的越界 panic。官方列為 Moderate；本報告 P1 是修正優先順序，不代表遠端程式碼執行。
- `govulncheck` 的一般與 `production` 掃描均報告受影響函式的實際呼叫鏈。
- 重現：在 `test_data/pivot-edit.xlsx` 的暫存副本中，僅將樞紐資料欄位 `fld` 改為 `99999`；經本專案 `document.Load` 開啟即出現 `index out of range [99999] with length 3`。
- 影響：不可信文件可使開檔流程 panic；從未受保護的 goroutine 開檔時可使 App 結束，連帶影響其他未儲存文件。沒有證據顯示此漏洞能執行任意程式碼。
- 建議：更新到包含 `6258dcebc4e2a2aed985c38a08098dfd908521d1` 修正或之後的相容版本；掃描器給出的第一個修補 pseudo-version 為 `v2.11.1-0.20261003002531-6258dcebc4e2`。同時加入畸形樞紐檔回歸及匯入錯誤隔離，避免僅依靠最外層 bindings 的 panic recovery。

### 2. P1：REPT 長度檢查的整數溢位可觸發 panic，並使文件鎖定（已修補）

- 位置：`internal/formula/functions/text.go:243`；`internal/service/session.go:201`；`internal/formula/engine.go:642`。
- `len([]rune(s))*n` 的乘法可能溢位，繞過 `maxText` 檢查；`strings.Repeat` 隨後 panic。
- 受控重現：32,768 個 ASCII 字元與 `n=562949953421312`。此數值仍小於 `Args.Int` 的 `1e15` 上限；長度乘積為 `2^64`，在 64-bit int 下回繞成零。
- `Engine.Rebuild` 實際出現 `strings: Repeat output length overflow`；再經公開的 `SheetService.SetCell` 測試，於測試程式 recover 後，另一個 goroutine 的 `Session.State()` 仍在 100ms 後阻塞。
- 原因：`Session.change` 在呼叫會計算公式的函式前上鎖，但未以 defer 解鎖；bindings 能 recover 不代表文件仍可使用。平行計算 worker 也沒有獨立的 panic 隔離。
- 建議：改用不會溢位的除法比較，例如非空字串時檢查 `n > maxText / runeCount`；補上文件鎖的異常釋放、計算錯誤隔離及狀態一致性處理。僅增加 recover 或 Unlock 不能保證已修改的文件／復原紀錄保持一致。

### 3. P2：檔案解壓及解析缺少適當資源預算（已修補）

- 位置：`internal/io/wwx/wwx.go:148`、`internal/io/xlsxfile/read.go:28`、`internal/io/xlsxfile/rawdrawings.go:256`。
- WWX 直接將 gzip 串流交給 JSON decoder，沒有應用層解壓後長度、單一儲存格長度及總資料量限制。受控測試的 16,522-byte 壓縮資料可正常載入 8,388,608-byte 單一儲存格字串。
- XLSX 使用 Excelize 預設解壓總量限制（目前依賴原始碼為 16GB）；自訂 XML／圖形零件另有 `io.ReadAll`，沒有較小的單零件預算。這不等於 XLSX 完全沒有任何解壓限制，但預設門檻仍足以讓桌面程式耗盡資源。
- 影響：惡意壓縮資料或超大欄位可消耗大量記憶體與 CPU。此次僅使用約 8MB 的受控資料，未嘗試耗盡記憶體或卡死使用者 App。
- 建議：依產品支援的大檔規模，設定壓縮／解壓後總量、單零件、單欄位、資料筆數及公式複雜度的上限，並提供超限錯誤及可取消的匯入流程。

### 4. 防禦補強：測試專用 bindings 在一般模式仍可呼叫部分寫檔方法（已修補）

- 位置：`main.go:115`、`internal/service/benchservice.go:115`、`:161`、`:176`。
- `BenchService` 一律註冊；`SaveSnapshot`、`ReportBench` 未檢查是否為測試模式，`EmitBurst`／`RecalcChain` 也沒有模式及合理工作量限制。與已有模式檢查的 `SaveCopy`／`SavePDF` 不一致。
- 重現：在獨立測試程序設定 `WW_BENCH=0`、`WW_SELFTEST=0`，仍能成功寫入暫存目錄下的 snapshot 與 report。
- 邊界：這不是已證實的網路攻擊。攻擊者須先有能力呼叫 App 的 Go bindings；snapshot 檔名有 `filepath.Base` 限制，不能據此宣稱可任意路徑穿越寫檔。正常模式 report 也不會觸發 Quit。
- 建議：限制具副作用及高成本方法的模式與參數，或協同調整前端初始化後，只在測試建置／模式註冊測試服務。

## 相依套件掃描

- `govulncheck v1.9.0`，Go `1.27.3`，資料庫 `https://vuln.go.dev`，本次掃描資料庫更新時間為 `2026-11-05T23:49:57Z`。
- 掃描一般模式與 `-tags production`，兩者皆有 6 個不同公告的版本警示；其中只有 `GO-2026-6695` 報告 symbol-level 可達呼叫鏈。其餘 `GO-2026-5932`、`GO-2026-6685`、`GO-2026-6687`、`GO-2026-6860`、`GO-2026-6866` 未報告實際呼叫到受影響函式，應追蹤但不等同可觸發。
- `npm audit`：2 個受影響套件、對應 1 個 Low 公告 [GHSA-gcqx-w48m-w2h4](https://github.com/advisories/GHSA-gcqx-w48m-w2h4)，為建置工具鏈的 `deepmerge`，警示傳遞至 `@sveltejs/vite-plugin-svelte`。當次回覆未提供可用修補版本。
- `npm audit --omit=dev`：0 個已報告漏洞。這僅表示當次資料庫沒有報告正式 npm 相依的已知漏洞，不代表整個 App 已安全。

## 其他觀察與範圍限制

- 儲存格富文字透過 `textContent` 建立 DOM，編輯器貼上讀取 `text/plain`；沒有找到將儲存格直接當成 HTML 執行的路徑。
- 未找到公式執行 shell／VBA 的路徑；`HYPERLINK` 現階段僅回傳顯示文字。更新檢查使用 HTTPS 與 10 秒 timeout，下載前需使用者按鈕操作，未發現自動下載並執行更新檔的流程。
- 本專案未設定 CSP。現有列印 iframe 載入自行產生的 PDF，不應因 iframe 存在就宣稱有外部程式碼執行漏洞；仍可加入 CSP 與外部導覽限制作為補強。[Wails 官方發布說明](https://github.com/wailsapp/wails/releases/tag/v3.0.0) 提醒應將嵌入的 iframe 視為可存取 bindings 的內容。
- 已對 Git 追蹤內容執行常見私鑰、AWS access key 與 GitHub token 格式的檔名級掃描，沒有命中；不代表完整秘密掃描，也未檢查 `my_docs/` 等私人目錄。
- 未做完整 fuzzing、WebView 攻擊鏈驗證、作業系統漏洞掃描、所有格式／圖片／字型解碼器測試，亦未驗證所有發布安裝檔。

## 驗證紀錄

指令：

```sh
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -json ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -tags production -json ./...
cd frontend
npm audit --json --ignore-scripts
npm audit --omit=dev --json --ignore-scripts
```

重現程式已從專案的暫存目錄移除，保留於本機 `/tmp/ww-excel-security-audit/reproduce.go`；畸形測試檔為同目錄的 `pivot-index.xlsx`。原始 `test_data/pivot-edit.xlsx` 未修改。

受控重現輸出：

```text
service REPT: PANIC strings: Repeat output length overflow
session state: BLOCKED after recovered panic
bench snapshot write in normal mode: <nil>
bench report write in normal mode: <nil>
pivot document.Load: PANIC runtime error: index out of range [99999] with length 3
REPT integer overflow: PANIC strings: Repeat output length overflow
WWX accepted: compressed=16522 bytes, cell=8388608 bytes
```

掃描原始資料位於 `/tmp/ww-excel-govulncheck.json`、`/tmp/ww-excel-govulncheck-production.json`、`/tmp/ww-excel-npm-audit.json`、`/tmp/ww-excel-npm-audit-prod.json`。上述為修補前的審查紀錄；第 1–4 項已依下列結果修補；剩餘相依警示仍由 todo 第 17 項追蹤。


## 第 13 項修補完成（2026-11-06）

- Excelize 更新並固定至 `v2.11.1-0.20261003002531-6258dcebc4e2`，此版本包含 [官方修補](https://github.com/qax-os/excelize/commit/6258dcebc4e2a2aed985c38a08098dfd908521d1)；必要的間接相依 `github.com/xuri/efp` 隨之更新為 `v0.0.2`。
- 上游修補會略過無效索引；應用層另驗證實際 pivotFields／cacheFields 數量與 dataField 索引，將畸形定義回報為匯入錯誤，避免默默載入欄位不完整的樞紐表。
- XLSX 解析邊界攔截非預期 panic，失敗時關閉已取得的來源、清除部分回傳值；涵蓋不經 Wails bindings recovery 的命令列開檔流程。此邊界僅保護 XLSX 解析，不取代第 14 項的公式與文件鎖修補，亦不能攔截所有資源耗盡情況。
- 重現測試先在舊版本確認負索引、超大索引、空快取及快取欄位不足四種案例皆 panic，再確認修補後均回報錯誤。另直接呼叫依賴的 `GetPivotTables`，確認上游本身也不再 panic。
- 服務回歸確認開檔失敗不改動原有未儲存文件及另一份文件、不開新視窗、不寫最近檔案紀錄，且仍能繼續開啟合法文件、編輯與復原。
- 完整 Go 測試通過；一般桌面回歸 284／284、樞紐專用驗收 25／25 通過，涵蓋欄位、篩選、重新整理、復原／重做與 XLSX／WWX 存檔重開。
- 一般與 `production` 的 govulncheck 均無 GO-2026-6695、GO-2026-6685、GO-2026-6687 警示，也沒有 symbol-level 可達的已知相依漏洞。剩餘 GO-2026-5932、GO-2026-6860、GO-2026-6866 仍是未報告可達呼叫的版本警示，持續追蹤。
- 修補後原始掃描：`/tmp/ww-excel-pivot-vuln-default.json`、`/tmp/ww-excel-pivot-vuln-production.json`；回歸結果：`/tmp/ww-excel-pivot-security-final-tests.log` 與 `/tmp/ww-excel-pivot-security-qa/`。


## 第 14 項修補完成（2026-11-06）

- `REPT` 使用 `utf8.RuneCountInString` 計算既有語意下的字元數，透過 `n > maxText / chars` 檢查上限，避免乘法溢位。空字串、零次數、負數、分數次數與 Unicode 邊界均有回歸。
- 在 `Evaluator.Eval`／`Reference` 的單一公式邊界攔截可回復的 panic，回傳 `#VALUE!`。此邊界在實際求值的 goroutine 執行，因此平行 worker、直接求值及參照求值也受保護；其他健康公式與依賴公式可繼續計算。
- 公式異常轉成正常的錯誤值後，原有 command、依賴更新、文件變更追蹤及 undo／redo 流程可以完整完成，避免變更途中中斷而遺留不一致的記錄。
- `Session.change` 使用局部範圍的 defer 解鎖：非公式 callback 的未預期 panic 仍向外傳遞，但不遺留鎖；所有事件在解鎖後發送，允許事件處理同步查詢文件。本修補不宣稱能回滾任意非公式 command 的部分副作用。
- 已測試原始 32,768 字元／562949953421312 次數案例，以及由短公式產生同樣長度的案例；新增 8 個 golden 案例。
- 注入求值器 panic，確認 scalar／reference 求值回傳錯誤並恢復 array mode；平行測試包含至少 512 個異常公式及 512 個健康公式，另確認依賴的 IFERROR 與後續重算正常。
- 服務測試確認錯誤後狀態查詢不阻塞、編輯／儲存／復原／重做與存檔點一致，XLSX／WWX 重開仍保留錯誤公式及其他正常資料。
- 完整 Go 測試、相關 data race 檢查、246 項前端測試、型別檢查與正式建置通過。桌面 294／294 通過，其中新增 10 項真實編輯器與 bindings 的公式安全檢查，涵蓋畫面收到錯誤、後續編輯及兩種檔案格式的存讀。
- 公式複雜度、記憶體預算、致命的 stack overflow／OOM 不在本次 recover 保證內，仍由第 15 項處理。
- 驗證紀錄：`/tmp/ww-excel-rept-before.log`、`/tmp/ww-excel-formula-security-full.log`、`/tmp/ww-excel-formula-security-race.log`、`/tmp/ww-excel-formula-security-qa/general.md`；效能短測見 `docs/perf.md`。


## 第 15 項修補完成（2026-11-06）

- 新增共用 `importguard` 與 context 讀檔路徑。WWX 在解壓／JSON 大量配置前先檢查字串、深度、資料大小與記錄數；XLSX 使用同一份私有快照預檢所有 ZIP part，並在 archive/zip 配置前核對真實 central directory 記錄，避免只信任可偽造的宣告筆數。保留來源供存檔的 Excelize 路徑也改用明確 512 MiB 解壓預算。
- 涵蓋自訂 XML、圖形、儲存格文字／筆數、公式長度／tokens／巢狀深度、總公式量、圖形與圖片資源；一般公式中間陣列也有元素上限。具體門檻、相容性與範圍見 [import-limits.md](import-limits.md)。
- 加入「正在開啟檔案」視窗與取消按鈕（支援 Esc）；取消呼叫獨立於編輯佇列。服務以每視窗工作 ID 隔離匯入，取消／失敗／被新工作取代／視窗關閉都不能把晚到的文件套回原視窗；取消也不顯示過期的第三方解析錯誤。
- 只有成功解析及計算後才提交文件、更新最近檔案。失敗與取消清理暫存快照和已開啟的 Excelize 來源，保留原有資料、dirty、undo／redo 與其他文件。
- 受控重現原始 16,522-byte WWX 膨脹成 8 MiB 單一字串的案例，確認在完整 JSON 解碼前拒絕。測試涵蓋各種大小／深度／筆數邊界、偽造 ZIP 宣告數、畸形 XML／JSON、gzip checksum、讀取中取消、平行公式取消、工作競爭、關閉後晚到的結果及後續開檔；相關 race 檢查通過。
- 完整 Go 測試、246 項前端測試、型別檢查及正式建置通過。一般桌面 294／294、實際百萬格開檔取消流程 9／9、樞紐編輯與 XLSX／WWX 往返 25／25 通過；紀錄位於 `/tmp/ww-import-qa/`。
- 安全預檢有明確效能成本：百萬格讀取中位數由純解析約 0.984 秒增為 2.485 秒，完整開檔與計算單次約 3.05 秒，GC 後新增 heap 約 179.2 MiB。詳見 [perf.md](perf.md)。
- 取消為合作式；第三方無 context 的呼叫需等目前步驟返回，2 分鐘不是強制殺死函式的硬時限。此修補不宣稱每個底層解碼器均安全、整個 App 有嚴格 RSS 上限，或能攔截所有 fatal OOM／stack overflow；亦不包含 CSV 的同等串流預算。


## 第 16 項修補完成（2026-11-06）

- 保留 `BenchService` 的啟動模式查詢，所有探測、強制 GC、事件推送、公式重算、snapshot 與報告寫檔都在 Go 後端檢查模式。`SaveCopy`／`SavePDF` 維持僅限 self-test；正常視窗不能透過這些測試方法寫檔、推送事件、觸發重算或強制 GC。
- 模式與報告位置固定於服務建立時；沒有可從前端啟用模式的方法。`WW_BENCH=0`／`WW_SELFTEST=0` 不會啟用，之後更改環境變數也不會開啟既有一般服務的測試能力。
- 加入事件數／區塊數／乘積與序列化大小限制、公式鏈上限、報告與 PNG 大小／尺寸、檔名與 PDF 圖表參數檢查；高成本工作以 atomic gate 互斥，超額並行呼叫立即拒絕。明確預算見 [bench-security.md](bench-security.md)。
- 報告僅於獲准且成功寫入、未取消後才啟動測試結束流程，失敗不觸發 Quit。取消／錯誤／panic 都會透過 defer 釋放工作 gate；`SaveCopy` 的直接存檔分支也用 defer 解鎖。
- 前端啟動保留兩個模式查詢，依後端結果啟動測試；一般模式即使開啟效能 HUD 也不顯示量測按鈕。測試報告失敗不會重新觸發無限的 unhandledrejection 回報。
- 回歸先在舊實作確認一般模式仍能寫入 snapshot 並覆寫既有報告，再確認修補後原報告及目錄保持原樣。測試亦涵蓋未啟用的 nil 依賴不被存取、模式固定、負值／MaxInt／超限乘積、payload 與圖片尺寸、寫入失敗不 Quit、工作互斥、取消後恢復，以及合法 XLSX 副本／PDF／PNG／報告。
- 完整 Go、相關 race、前端 246 項、型別檢查與正式建置通過；桌面 self-test 300／300（新增 6 項真實 bindings 驗證）及完整 benchmark 通過。既有 200×0、200×1、50×9 事件均完整收到，1,000／10,000／100,000 格公式鏈、記憶體、snapshot 與報告皆正常。
- 本次沒有更新相依版本或重跑漏洞資料庫；第 17 項的相依警示持續保留。本限制在 Go 方法收到參數後生效，不是 RPC 傳輸大小上限，也不是整個 WebView 的 sandbox。
- 紀錄：`/tmp/ww-bench-security-before.log`、`/tmp/ww-bench-security-go.log`、`/tmp/ww-bench-security-race.log`、`/tmp/ww-bench-security-qa/`。


## 第 17 項重新掃描與處理建議（2026-11-06，第 16 項完成後）

本次只重新掃描、查詢修補版本並更新紀錄，沒有升級套件、修改 lockfile 或重建 App。

| 掃描 | 結果 |
| --- | --- |
| npm audit（含 dev） | 2 個 Low 套件，對應同一則 GHSA-gcqx-w48m-w2h4；沒有 Moderate／High／Critical |
| npm audit --omit=dev | 0 個警示 |
| govulncheck 一般模式 | 3 個不同公告，0 個 symbol-level 可達警示 |
| govulncheck production | 同上，沒有新增可達呼叫鏈 |

Go 掃描使用 Go 1.27.3、govulncheck v1.9.0，資料庫更新時間仍為 `2026-11-05T23:49:57Z`。兩種模式均正常結束（exit 0）；JSON 的 OSV 記錄包含歷史公告，結果只計算 `finding` 引用的不同公告 ID，避免把所有資料庫回傳記錄當成命中。

| 公告／相依 | 目前版本與可達性 | 後續建議 |
| --- | --- | --- |
| [GHSA-gcqx-w48m-w2h4](https://github.com/advisories/GHSA-gcqx-w48m-w2h4)，deepmerge | 4.3.1；經 @sveltejs/vite-plugin-svelte 7.3.1 引入，限 dev 工具鏈。npm audit 的 fixAvailable=false，npm registry 最新版仍為 4.3.1，官方公告列 Patched versions: None | 等待上游修補並持續追蹤 |
| [GO-2026-6860](https://pkg.go.dev/vuln/GO-2026-6860)／[GO-2026-6866](https://pkg.go.dev/vuln/GO-2026-6866)，golang.org/x/net | v0.60.0；html 套件有匯入，但掃描沒有找出 Parse／Render 等受影響函式的可達呼叫鏈 | 官方列出的 v0.62.0 可取得，可安排一般維護升級與回歸 |
| [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932)，golang.org/x/crypto | v0.57.0；警示針對不再維護的 openpgp，只有 module-level 命中，沒有受影響套件／函式的可達紀錄 | 掃描器列出的第一個不受影響版本為 v0.58.1-0.20261009181718-2f1f834330b2，可隨 Go 相依維護評估 |

以上兩個 Go 候選版本的 go.mod 均要求 Go 1.26.0，目前專案 Go 1.27.3 符合。x/net v0.62.0 同時要求 x/crypto v0.58.0、x/sys v0.49.0、x/text v0.43.0 等，實際升級仍需檢查最終版本選擇與回歸；本次未據此宣稱已通過升級相容性測試。openpgp 的公告涉及棄用及移除套件，不能解讀為原有 OpenPGP 實作已修好。

本機 vite-plugin-svelte 的 deepmerge 呼叫位於建置設定的 `mergeConfigs`，合併預設值與 Svelte／Vite 設定；本次沒有找到將活頁簿內容送入這個建置路徑的流程。結合正式 npm 掃描為零與公告的 Low 分級，評估可等待上游，不需要為清空警示而強制降版或覆寫相依。

結論：第 17 項保留為低優先維護。Go 相依已有可取得的候選版本，可安排升級與測試；前端項目目前以追蹤為主。此次沒有發現需要立即修補的新可達漏洞，也不代表整個 App 已無漏洞。

掃描紀錄：`/tmp/ww-excel-rescan-npm.json`、`/tmp/ww-excel-rescan-npm-prod.json`、`/tmp/ww-excel-rescan-go.json`、`/tmp/ww-excel-rescan-go-prod.json`；版本查詢：`/tmp/ww-excel-rescan-deepmerge-latest.json`、`/tmp/ww-excel-rescan-go-fixes.json`。npm audit 含 dev 的 exit 1 表示有警示，非掃描失敗。
