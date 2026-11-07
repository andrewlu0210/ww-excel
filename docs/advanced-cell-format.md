# 斜線框線、圖樣與漸層填滿

2026-11-07，第 23 項。以第 18–22 項的共用草稿、混合格式、一次套用／復原為基礎，依序完成斜線、圖樣與漸層。

## 操作

選取儲存格後按 ⌘1，或使用右鍵的「儲存格格式」。

### 斜線框線

「框線」頁勾選「斜線 ╱」或「斜線 ╲」，可同時勾選形成交叉。新增斜線使用目前線型／線條顏色；「套用斜線線型／色彩」可更新既有斜線。依 Excel 格式，兩個方向共用同一種線型與顏色。

清除其中一個方向不會更改另一方向的樣式；「無框線」會一併清除斜線。斜線只屬於所選儲存格，沒有四邊框線的相鄰鏡射規則。合併儲存格橫跨整個合併區繪製。

斜線與斜點畫線不同：斜線是儲存格內的 ╱／╲；斜點畫線是一種可用於四邊或斜線的線型。

### 圖樣填滿

「填滿」頁將「填滿類型」選為「圖樣填滿」，選擇圖樣及前景／背景色。提供 Excel 的 17 種非單色圖樣，包括五種灰階密度、深／淺水平、垂直、向上／向下斜線與格線。

「填滿色彩」與「無填滿」會取代進階填滿；白色單色仍與無填滿不同。只修改字型、數值格式或框線時，原有進階填滿保持不變。

### 漸層填滿

- 「線性漸層」：設定 0–360° 角度。
- 「矩形路徑漸層」：設定起始矩形的左／右／上／下座標，皆為 0–1，左不可大於右、上不可大於下。中心向外可使用四個值均為 0.5。
- 色彩節點包含 0–1 位置及色彩，可新增／移除，至少保留兩個；編輯器可新增至 16 個，匯入與儲存保留最多 256 個節點。

位置、角度與範圍不合法時無法確定。預覽會隨草稿更新；切換分頁保留草稿，取消／Esc 不修改文件。「重設變更」恢復開啟時的狀態。多選會區分共同與混合填滿，只套用實際修改的屬性。

## 顯示與儲存

- Canvas、填滿預覽與 PDF 使用相同圖樣、色彩插值及漸層座標規則；線性角度會考慮儲存格寬高。斜線在 SVG 預覽、Canvas 與 PDF 支援既有線型與顏色。
- 直接進階填滿優先於表格底色；條件格式填滿仍可覆蓋。格式視窗編輯底層樣式，保留條件格式設定。
- 模型以可比較的 `FillSpec` 及正規化節點資料維持樣式去重。WWX 為新增欄位，仍可讀取既有只含單色填滿／四邊框線的舊檔。
- XLSX 直接轉換完整的 patternFill／gradientFill，保留兩種圖樣色彩、任意漸層角度、節點位置及矩形參數。避免使用會簡化這些資訊的 Excelize 一般 Fill 轉換；僅改字型後再存檔亦有往返測試。
- 整列／整欄保持稀疏；一次復原／重做會還原完整格式。工具列單色填滿及清除框線操作也會正確處理新增欄位。

圖樣與漸層在 Canvas／PDF 採有界點陣繪製，快取有數量上限；極大合併區放大後可能看見取樣差異。Excel 使用自己的圖樣密度、線寬與字型度量，因此畫面／PDF 不保證逐像素相同，XLSX 的圖樣名稱、色彩與漸層參數則完整往返。此功能不加入色彩透明度。

## 驗收證據

代表案例依序包含 17 種圖樣、37° 三節點線性漸層與矩形路徑漸層，每格都有紅色中點畫交叉斜線。

- [原始 XLSX](advanced-cell-format/advanced.xlsx)、[WWX](advanced-cell-format/advanced.wwx)、[僅改字型後的 XLSX](advanced-cell-format/advanced-edited.xlsx)。
- [Microsoft Excel 16.114.3 另存 XLSX](advanced-cell-format/advanced-excel.xlsx)、[Excel 讀取結果](advanced-cell-format/excel-report.txt)。
- [應用程式 PDF](advanced-cell-format/advanced-app.pdf)、[Excel PDF](advanced-cell-format/advanced-excel.pdf)。
- [應用程式頁面](advanced-cell-format/app.png)、[Excel 頁面](advanced-cell-format/excel.png)。

已用真正 Microsoft Excel 開啟、讀取圖樣及兩條斜線的線型、另存 XLSX 與 PDF，再由應用程式讀取 Excel 另存檔。全部 19 種案例的填滿參數、字型、兩條斜線及顏色均通過程式比對；PDF 已目視檢查。過程僅操作合成驗收文件。

## 回歸與重現

```sh
WW_ADVANCED_FORMAT_OUT=/tmp/ww-advanced-format \
  go test ./internal/service -run TestAdvancedFormat -count=1

WW_ADVANCED_EXCEL_SAVED="$PWD/docs/advanced-cell-format/advanced-excel.xlsx" \
  go test ./internal/service -run TestAdvancedFormatRoundTrip -count=1

go test ./...
go test -race ./internal/model ./internal/command ./internal/service \
  ./internal/io/xlsxfile ./internal/io/wwx ./internal/printing
npm --prefix frontend run check
npm --prefix frontend test
npm --prefix frontend run build

WW_SELFTEST=1 WW_BENCH_OUT=/tmp/ww-advanced-desktop/report.md ./bin/ww-excel
```

Go 包含圖樣密度、角度／路徑／插值、參數拒絕、合併、混合值、稀疏整欄、條件格式優先序、往返與 PDF 真正斜向筆畫測試。前端另驗證 Canvas 單格／合併區的兩個斜線方向與深色模式的自動文字對比。


## 最終結果

- 前端型別檢查 0 errors／0 warnings，282 項測試與正式建置通過。
- 完整 Go 測試與 model／command／service／xlsxfile／wwx／printing 的 race 檢查通過；雙線斜線間距修正後 printing 回歸亦通過。
- [最終通用版桌面報告](advanced-cell-format/desktop-report.md)：一般 308／308、格式視窗 14／14、框線 18／18、填滿 20／20、混合格式 24／24、相容性 10／10、進階格式 16／16，合計 **410／410**。
- `bin/ww-excel` 已更新 Apple Silicon／Intel 通用版，ad-hoc 簽章及 `codesign --verify --strict` 通過；以上最終桌面驗收直接使用這份交付程式。
