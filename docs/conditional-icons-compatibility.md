# 條件格式圖示集

已支援 Microsoft [ST_IconSetType 規格](https://learn.microsoft.com/en-us/openspecs/office_standards/ms-xlsx/bfdf00a5-4e49-4b00-8803-32250e1683c2) 的 20 組圖示，以 Canvas 路徑繪製，不依賴系統符號字型。

| 類別 | 圖示集 |
| --- | --- |
| 箭頭 | 3Arrows、3ArrowsGray、4Arrows、4ArrowsGray、5Arrows、5ArrowsGray |
| 旗標 | 3Flags |
| 交通燈與形狀 | 3TrafficLights1、3TrafficLights2、4TrafficLights、3Signs、4RedToBlack、3Triangles |
| 符號 | 3Symbols、3Symbols2 |
| 評等 | 4Rating、5Rating、5Quarters、3Stars、5Boxes |

選取儲存格後，從「格式 → 設定格式化的條件」新增「圖示集」規則。可選圖示、反向順序、只顯示圖示，並設定每級門檻的 ≥／> 與數值、百分比、百分位數或公式。百分比依範圍最大／最小值計算，百分位數依資料分布插值。最低級包含未達其他門檻的數值。

匯入的逐級自訂圖示與 NoIcons 會保留並顯示；目前介面提供標準圖示集新增與規則刪除，尚未提供逐級自訂圖示選擇器。未知或無效圖示仍顯示數值。只顯示圖示、自訂無圖示的空白、窄欄與合併儲存格均保留原本行為。

## 驗證

- 前端 188 項測試及型別檢查通過；涵蓋全部圖示狀態、顯示模式、錯誤圖示、窄欄與合併儲存格。
- 完整 Go 測試通過；涵蓋門檻與相等邊界、百分位數、公式、反向、自訂圖示、NoIcons、驗證錯誤、修改數值與復原／重做，以及全部 20 組的 XLSX／WWX 往返。
- 原生桌面程式 169／169 項檢查通過；包含開啟、新增圖示規則、五級預覽、修改與復原／重做，以及 XLSX → WWX → XLSX 另存重開。
- 已檢查實際網格、自訂門檻、窄欄與圖示總覽截圖。Canvas 自繪圖形並非 Excel 點陣圖逐像素複製；2026-11-03 使用者回報以 Microsoft Excel 開啟正常。

[圖示總覽](snapshot-conditional-icons-gallery.png) · [測試活頁簿](../test_data/conditional-icons.xlsx)

## 重現

```sh
GOCACHE=/private/tmp/ww-excel-go-build go run ./tools/iconfixture -o test_data/conditional-icons.xlsx
WW_SELFTEST=1 WW_XLSX_FIXTURE="$PWD/test_data/conditional-icons.xlsx" WW_BENCH_OUT=/private/tmp/ww-excel-icons/report.md ./bin/ww-excel
```

產生器建立三張工作表：20 組圖示的兩種顯示模式、門檻與自訂圖示、窄欄與合併。桌面自測在同目錄另存測試副本，保留來源檔案。
