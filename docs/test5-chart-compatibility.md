# test5.xlsx「9.圖表」顯示相容性

2026-11-03，以使用者提供的 Excel「會議出席」截圖、原始 XLSX DrawingML 設定及本機 Wails 視窗擷取結果對照。測試檔位於 `test_data/test5.xlsx`，不納入 Git。

## 本次修正

- 數值軸自動範圍保留資料外側留白：最高值 20000 時，顯示 0–25000，每格 5000；明確指定的上下限維持原值。
- 讀取原檔的 219% 柱距，取代固定柱寬；保留零柱距與另存 XLSX 後的設定。
- 讀取日期類別軸的字型、11pt 字級及淺灰色軸線。Windows 字型沿用既有 macOS 對應機制。
- 圖形文字保留 DrawingML `a:br` 換行，水平與垂直座標軸的括號說明各自顯示在下一行。
- 多組座標軸的文件暫時保留第一組軸，避免將主要柱狀數列誤套次座標軸的金額格式與隱藏日期標籤設定。

## 驗證

- `go test ./internal/...`：通過。
- `cd frontend && npm run check`：0 errors、0 warnings。
- `cd frontend && npm test`：145 項通過。
- 桌面唯讀測試：11 / 11 項通過。實際開啟 test5.xlsx、切換「9.圖表」，檢查數據、年份、座標軸範圍、柱距、圖形換行與 Canvas；原文件保持未修改。

重跑桌面檢查：

```sh
WW_SELFTEST=1 \
WW_XLSX_FIXTURE="$PWD/test_data/test5.xlsx" \
WW_BENCH_OUT="$PWD/docs/selftest-latest.md" \
./bin/ww-excel
```

測試報告及 `test5-chart-<id>.png`、`test5-attendance-comparison.png`、`test5-chart-sheet.png` 會寫在報告所在目錄。Canvas 擷取不含 macOS 標題列與功能區。

## 雙軸組合圖

分頁下方的柱狀＋折線雙軸組合圖已完成本機支援與回歸檢查，詳見 [雙軸組合圖檢查](test5-combo-compatibility.md)。

## 尚未涵蓋
- 未以真正 Excel 重開修正版輸出的檔案；字型替代、字距及版面仍可能與 Windows Excel 略有差異。
