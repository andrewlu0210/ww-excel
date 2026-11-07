# 儲存格格式相容性驗收

2026-11-07，第 22 項。第一階段格式視窗的操作方式見 [格式視窗](cell-format-dialog.md)、[框線](border-editing.md)、[填滿](fill-editing.md)及[混合格式](mixed-cell-format.md)。

## 驗收案例

`internal/service/format_acceptance_test.go` 建立單格、多格、不連續、重疊、合併區內部選取、整欄及整列七種案例。每種案例有既有列／欄樣式、字型與實際內容，再同時修改字級、填滿、對齊、數值格式及不同線型／顏色的外框與內框。

檢查未選範圍與未修改字型、一次復原／重做、稀疏區塊數及 XLSX／WWX 往返。整欄包含第 1,048,576 列，整列包含第 16,384 欄；合併區由內部非錨點格觸發。既有前端／桌面案例另涵蓋取消、混合值、清除邊線、無填滿與白色、表格及條件格式優先序、手動與自動列高。

`internal/io/xlsxfile/axisstyles_write_test.go` 檢查列欄 XML 的範圍拆分、寬度／隱藏／列高保留、列格式移除、空白工作表、遠端列格式及連續儲存。稀疏整欄匯出不可實體化百萬個空白儲存格，第二次儲存也不可遺失列欄格式。

## 真正 Microsoft Excel 比對

以本機 Microsoft Excel **16.114.3** 開啟合成檔，透過其 AppleScript 物件模型讀取格式，再由 Excel 原生另存 XLSX 與 PDF。僅操作專用測試文件，保留原先已開啟的使用者文件。

- [原始 XLSX](format-acceptance/formats.xlsx)、[原始 WWX](format-acceptance/formats.wwx)、[Excel 另存 XLSX](format-acceptance/formats-excel.xlsx)。
- [應用程式 PDF](format-acceptance/formats-app.pdf)、[Excel PDF](format-acceptance/formats-excel.pdf)。
- [Excel 實際格式讀值](format-acceptance/excel-report.txt)。RGB 清單在 AppleScript 轉為文字時會連接，例如 `2552550` 為 `{255,255,0}`；合併範圍的 address 在 Excel AppleScript 回傳 missing value，改以另存檔的合併資訊驗證。
- [應用程式頁面](format-acceptance/app.png)、[Excel 頁面](format-acceptance/excel.png)。兩者皆為一頁；已目視比對黃色數值、合併填滿、無填滿／白色／橘色、紅色雙外框、藍色中點畫內框及十三種線型。字型度量、縮放比例、線寬與虛線節奏仍有渲染器差異，驗收不要求逐像素相同。

`TestFormatAcceptanceFixture` 重新讀取 Excel 另存檔，比對數值、字型、字級、粗體、數值格式、填滿、合併及十三種框線的線型與顏色，全部通過。此項確認的是儲存格格式，不代表尚未實作的圖樣／漸層填滿或斜線框線互通已完成。

另外用 Excel 開啟[桌面整列整欄案例](format-acceptance/axes.xlsx)，確認第 1,048,576 列、第 16,384 欄、列欄交點與合併錨點；[實際讀值](format-acceptance/excel-axes-report.txt)及[原生另存檔](format-acceptance/axes-excel.xlsx)已保留。`TestFormatAcceptanceNativeAxes` 重開原生另存檔，檢查填滿、繼承 Arial 斜體與合併範圍，全部通過。

## 驗收發現及修正

1. 選到合併區內部格時，一般格式原先未展開到錨點。格式與框線現在共同使用展開後的合併範圍，自動列高驗證也使用相同範圍。
2. PDF 原先把不同點畫線簡化為通用虛線，且中粗變體遺失線寬。現在保留各種點畫節奏與線寬；斜點畫線在 Canvas、SVG 預覽與 PDF 都繪製斜向線段。
3. Excelize 的整欄樣式操作會依最後一列實體化大量空白格，使應用程式匯出的檔案超過自身匯入限制。改為稀疏列欄 XML 格式，僅對實際存在的儲存格投影繼承格式，保留其他列欄屬性及未修改的 XML。
4. XLSX 的縮放列印缺少 `fitToPage` 啟用旗標，Excel 會忽略一頁寬高設定。現在依縮放模式同步旗標；實際 Excel PDF 已從兩頁恢復一頁。

## 驗證結果

- 完整 Go 測試及 command／service／xlsxfile／printing 的 race 檢查通過。
- 前端型別檢查 0 errors／0 warnings，274 項測試與正式建置通過。
- [原生桌面報告](format-acceptance/desktop-report.md)：一般 308／308、格式視窗 14／14、框線 18／18、填滿 20／20、混合格式 24／24、相容性 10／10，合計 **394／394**。

Apple Silicon／Intel 通用版 `bin/ww-excel` 已重新建置，ad-hoc 簽章及 `codesign --verify --strict` 驗證成功。

## 重現

```sh
WW_FORMAT_ACCEPTANCE_OUT=/tmp/ww-format-acceptance \
  go test ./internal/service -run TestFormatAcceptanceFixture -count=1

WW_FORMAT_EXCEL_SAVED="$PWD/docs/format-acceptance/formats-excel.xlsx" \
  go test ./internal/service -run TestFormatAcceptanceFixture -count=1

WW_FORMAT_EXCEL_AXES="$PWD/docs/format-acceptance/axes-excel.xlsx" \
  go test ./internal/service -run TestFormatAcceptanceNativeAxes -count=1

go test ./...
go test -race ./internal/command ./internal/service ./internal/io/xlsxfile ./internal/printing
npm --prefix frontend run check
npm --prefix frontend test
npm --prefix frontend run build

WW_SELFTEST=1 WW_BENCH_OUT=/tmp/ww-format-acceptance-desktop/report.md ./bin/ww-excel
```

桌面報告包含一般回歸、格式視窗、框線、填滿、混合格式與格式相容性六個區段。新的相容性區段操作真正格式視窗，檢查合併錨點、一次復原／重做、整列整欄邊界、列欄交點、繼承字型及兩種格式重開。
