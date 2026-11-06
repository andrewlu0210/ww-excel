# 測試 bindings 的模式與工作量限制

`BenchService` 仍註冊於一般視窗，讓 App 啟動時可查詢 `BenchMode`／`SelfTestMode`；具副作用或計算成本的方法都在 Go 後端拒絕一般模式呼叫。`FixturePath` 在非 self-test 模式回傳空字串，不洩漏測試路徑。

模式與輸出位置在建立服務時由 `WW_BENCH=1`、`WW_SELFTEST=1`、`WW_BENCH_OUT` 讀取並固定；前端沒有開啟模式的方法，之後更改環境變數也不會把一般服務變成測試服務。`WW_BENCH=0` 或其他非 `1` 的值不會啟用。

| 方法 | 一般模式 | benchmark | self-test |
| --- | --- | --- | --- |
| BenchMode／SelfTestMode | 可查詢 | 可查詢 | 可查詢 |
| FixturePath | 空字串 | 空字串 | 可讀取設定路徑 |
| Ping／Mem／EmitBurst／RecalcChain | 拒絕 | 允許，受限額約束 | 允許，受限額約束 |
| SaveSnapshot／ReportBench | 拒絕 | 允許，受限額約束 | 允許，受限額約束 |
| SaveCopy／SavePDF | 拒絕 | 拒絕 | 允許，受限額約束 |

只有成功寫入獲准的報告後才啟動測試結束流程；一般模式、超限、取消或寫檔失敗均不會因此觸發 Quit。一般模式的 `Mem` 也不能強制 GC。前端會查詢後端模式；即使開啟效能 HUD，一般模式也不顯示量測按鈕。

| 工作 | 上限與驗證 |
| --- | --- |
| 事件推送 | 每次 1–1,000 個事件；每事件 0–16 個區塊；事件數 × 區塊數 ≤1,000 |
| 事件資料 | 每事件含預留 envelope ≤2 MiB；總量 ≤32 MiB，先保守估計文字與富文字，再序列化核對；超限不發送任何事件 |
| 公式鏈 | 1–100,000 格；建立與初算有 context 檢查、30 秒取消期限，最後計時的更新仍使用既有引擎，於返回時確認取消狀態 |
| Markdown 報告 | ≤1 MiB |
| PNG snapshot | base64 長度在解碼前檢查；解碼後 ≤8 MiB；PNG header 可讀，寬高各 ≤8,192，像素 ≤16,000,000 |
| PDF 圖表輸入 | ≤128 張、data URL 合計 ≤24 MiB；每張使用相同的 PNG 資料／尺寸限制，工作表及圖表 ID 不可為負值 |
| 輸出檔名 | 原始名稱 ≤120 bytes，拒絕空字串、`.`、`..`、斜線、反斜線及 NUL；固定輸出至報告所在目錄 |
| 並行工作 | 每個 App 同時最多一項高成本探測或輸出操作；其他請求立即回報 busy，不建立等待佇列。Ping 與模式查詢不佔此名額 |

MiB 為 1,048,576 bytes。測試輸出新檔使用 0600 權限。snapshot 只驗證 PNG header／尺寸以限制資料與顯示規模，不完整解碼每個像素來驗證圖片內容。服務端限制於 RPC 參數完成解碼後生效，不是傳輸層的請求大小上限。取消為合作式，已進行中的檔案寫入、GC、PDF 渲染等步驟不能保證即時中斷。

這些是測試 API 的防禦限制，沒有變更其他正式功能的授權規則。明確啟用的測試模式仍可依啟動時的輸出設定寫檔及結束 App，不能當成隔離不可信前端的 sandbox。

既有 benchmark 的 200×0、200×1、50×9 事件測試與 1,000／10,000／100,000 格重算均在限額內。驗證指令：

```sh
go test ./internal/service -run '^Test(Bench|ChartRewrite)'
go test -race ./internal/service -run '^Test(Bench|ChartRewrite)'
WW_SELFTEST=1 WW_BENCH_OUT=/tmp/ww-bench-security-qa/general.md ./bin/ww-excel-display-fix
WW_BENCH=1 WW_BENCH_OUT=/tmp/ww-bench-security-qa/benchmark.md ./bin/ww-excel-display-fix
```
