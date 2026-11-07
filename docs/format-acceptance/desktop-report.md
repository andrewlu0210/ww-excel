# Self-test

- 時間：2026-11-07T06:59:53.009Z
- 結果：308 / 308 通過

| | 項目 | 細節 |
| --- | --- | --- |
| ✅ | 輸入時進入「輸入」模式 | editing = {"mode":"enter","pos":{"row":0,"col":0}} |
| ✅ | Enter 提交 | A1 = "Hello"，預期 "Hello" |
| ✅ | Enter 下移 | 作用中 A2，預期 A2 |
| ✅ | 數字 | A2 = "123"，預期 "123" |
| ✅ | 中文文字 | B2 = "台北"，預期 "台北" |
| ✅ | Tab 後 Enter 回到起始欄 | 作用中 A3，預期 A3 |
| ✅ | 前端快取收到 diff（數字靠右） | cache(A2) = {"text":"123","numeric":true,"center":false,"style":0,"color":null,"bar":null} |
| ✅ | F2 進入「編輯」模式 | editing = {"mode":"edit","pos":{"row":0,"col":0}} |
| ✅ | 編輯模式方向鍵不提交 | editing = {"mode":"edit","pos":{"row":0,"col":0}} |
| ✅ | F2 編輯後提交 | A1 = "Hello!"，預期 "Hello!" |
| ✅ | Esc 取消編輯 | A2 = "123"，預期 "123" |
| ✅ | Esc 後回到就緒 |  |
| ✅ | ⌘Z 復原 | A1 = "Hello"，預期 "Hello" |
| ✅ | 復原後選取受影響儲存格 | 作用中 A1，預期 A1 |
| ✅ | ⇧⌘Z 重做 | A1 = "Hello!"，預期 "Hello!" |
| ✅ | ⌘Enter 填滿 A5 | A5 = "x"，預期 "x" |
| ✅ | ⌘Enter 填滿 B6 | B6 = "x"，預期 "x" |
| ✅ | ⌘Enter 保留選取範圍 | [{"r0":4,"c0":0,"r1":5,"c1":1}] |
| ✅ | 選取範圍內 Enter 循環 | 作用中 A6，預期 A6 |
| ✅ | Delete 清除 | B6 = ""，預期 "" |
| ✅ | 清除後復原 | B6 = "x"，預期 "x" |
| ✅ | ⌘↓ 到連續資料尾端 | 作用中 A2，預期 A2 |
| ✅ | ⌘↓ 跳到下一段資料 | 作用中 A5，預期 A5 |
| ✅ | ⇧⌘↓ 延伸選取 | {"r0":4,"c0":0,"r1":5,"c1":0} |
| ✅ | ⌥Enter 換行 | D1 = "line1\nline2"，預期 "line1\nline2" |
| ✅ | 輸入模式方向鍵提交 | E1 = "7"，預期 "7" |
| ✅ | 輸入模式方向鍵移動 | 作用中 F1，預期 F1 |
| ✅ | 單引號保留文字 | F1 = "'007"，預期 "'007" |
| ✅ | ⇧Space 選整列 | {"r0":1,"c0":0,"r1":1,"c1":16383} |
| ✅ | ⌘A 全選 | {"r0":0,"c0":0,"r1":1048575,"c1":16383} |
| ✅ | 函數自動完成 | ["SUBSTITUTE","SUBTOTAL","SUM","SUMIF","SUMIFS","SUMPRODUCT","SUMSQ","SUMX2MY2","SUMX2PY2","SUMXMY2"] |
| ✅ | 繼續輸入縮小清單 | ["SUM","SUMIF","SUMIFS","SUMPRODUCT","SUMSQ","SUMX2MY2","SUMX2PY2","SUMXMY2"] |
| ✅ | Tab 接受建議並顯示參數提示 | {"suggestions":[],"hint":"SUM","error":null,"highlights":[]} |
| ✅ | 方向鍵插入參照（指向模式） | [{"range":{"r0":8,"c0":0,"r1":8,"c1":0},"color":0,"dashed":true}] |
| ✅ | 指向模式產生 =SUM(A8:A9) | A10 = "=SUM(A8:A9)"，預期 "=SUM(A8:A9)" |
| ✅ | 公式結果 | cache(A10) = {"text":"7","numeric":true,"center":false,"style":0,"color":null,"bar":null} |
| ✅ | 修改參照儲存格後重算 | cache(A10) = {"text":"14","numeric":true,"center":false,"style":0,"color":null,"bar":null} |
| ✅ | F4 切換為 $A$8 | B10 = "=$A$8"，預期 "=$A$8" |
| ✅ | 滑鼠點選插入參照 | D10 = "=A8*2"，預期 "=A8*2" |
| ✅ | 滑鼠點選後的結果 | {"text":"20","numeric":true,"center":false,"style":0,"color":null,"bar":null} |
| ✅ | 公式標準化並自動補上括號 | E10 = "=ROUND(A8/3,1)"，預期 "=ROUND(A8/3,1)" |
| ✅ | 公式錯誤時保持編輯並提示 | "這裡缺少「)」或「,」（位置 6）" |
| ✅ | 錯誤公式不寫入 | F10 = ""，預期 "" |
| ✅ | 錯誤值置中 | {"text":"#DIV/0!","numeric":false,"center":true,"style":0,"color":null,"bar":null} |
| ✅ | 邏輯值置中 | {"text":"TRUE","numeric":false,"center":true,"style":0,"color":null,"bar":null} |
| ✅ | 公式列顯示公式 | "=SUM(A8:A9)" |
| ✅ | 點選公式列進入編輯模式 | {"mode":"edit","pos":{"row":9,"col":0}} |
| ✅ | 循環參照回報 | "A12" |
| ✅ | 循環參照公式保留 | A12 = "=A12+1"，預期 "=A12+1" |
| ✅ | 數值格式與粗體、填色 | {"v":"1,234.50","style":{"id":2,"font":{"name":"Calibri","size":11,"bold":true},"fill":"FFF2CC","left":{},"right":{},"top":{},"bottom":{},"numFmt":"#,##0.00"}} |
| ✅ | 前端樣式表同步 |  |
| ✅ | 輸入日期自動套用日期格式 | {"v":"2026/10/23","style":{"id":3,"font":{"name":"Calibri","size":11},"left":{},"right":{},"top":{},"bottom":{},"numFmt":"yyyy/m/d"}} |
| ✅ | 輸入百分比 | {"v":"12%","style":{"id":4,"font":{"name":"Calibri","size":11},"left":{},"right":{},"top":{},"bottom":{},"numFmt":"0%"}} |
| ✅ | 輸入貨幣 | {"v":"$1,234 ","style":{"id":5,"font":{"name":"Calibri","size":11},"left":{},"right":{},"top":{},"bottom":{},"numFmt":"\"$\"#,##0_);[Red]\\(\"$\"#,##0\\)"}} |
| ✅ | 日期放不下時自動加寬欄 | {"i":1,"px":86} |
| ✅ | 修改值時保留格式 | {"v":"99.00","style":{"id":2,"font":{"name":"Calibri","size":11,"bold":true},"fill":"FFF2CC","left":{},"right":{},"top":{},"bottom":{},"numFmt":"#,##0.00"}} |
| ✅ | 外框線只套在邊緣 | {"corner":{"id":8,"font":{"name":"Calibri","size":11},"left":{"style":"thin"},"right":{},"top":{"style":"thin"},"bottom":{}},"middle":{"id":6,"font":{"name":"Calibri","size":11},"left":{},"right":{},"top":{"style":"thin"},"bottom":{}}} |
| ✅ | 跨欄置中 | [{"r0":19,"c0":0,"r1":19,"c1":2}] |
| ✅ | 從合併儲存格往右移跳過整個合併 | {"row":19,"col":3} |
| ✅ | 調整欄寬 | [{"i":4,"px":120},{"i":1,"px":86}] |
| ✅ | 放大字型後自動加高列 | [{"i":0,"px":41},{"i":14,"px":23}] |
| ✅ | 換行自動開啟自動換行 | {"v":"第一行\n第二行","style":{"id":1,"font":{"name":"Calibri","size":11},"left":{},"right":{},"top":{},"bottom":{},"wrap":true}} |
| ✅ | 換行後自動加高列 | {"i":21,"px":41} |
| ✅ | 新增工作表 | ["工作表1","工作表2"] |
| ✅ | 跨工作表公式 | {"v":"42","style":{"id":0,"font":{"name":"Calibri","size":11},"left":{},"right":{},"top":{},"bottom":{}}} |
| ✅ | 改名後公式跟著更新 | E15 = "=報表!A1"，預期 "=報表!A1" |
| ✅ | 改名成功 | null |
| ✅ | 不合法的名稱被拒絕 |  |
| ✅ | 刪除後參照變成 #REF! | {"v":"#REF!","style":{"id":0,"font":{"name":"Calibri","size":11},"left":{},"right":{},"top":{},"bottom":{}}} |
| ✅ | 復原刪除工作表 | ["工作表1","報表"] |
| ✅ | xlsx 存讀：數值格式與粗體 | {"v":"99.00","style":{"id":2,"font":{"name":"Calibri","size":20,"bold":true},"fill":"FFF2CC","left":{},"right":{},"top":{},"bottom":{},"numFmt":"#,##0.00"}} |
| ✅ | xlsx 存讀：合併與欄寬 | [{"i":1,"px":86},{"i":4,"px":120}] |
| ✅ | xlsx 存讀：工作表與跨表公式 | ["工作表1","報表"] |
| ✅ | xlsx 存讀：日期與換行 |  |
| ✅ | xlsx 存讀：貨幣格式 | {"v":"$1,234 ","style":{"id":5,"font":{"name":"Calibri","size":11},"left":{},"right":{},"top":{},"bottom":{},"numFmt":"\"$\"#,##0_);[Red]\\(\"$\"#,##0\\)"}} |
| ✅ | ⌘C 顯示複製虛線框 | {"r0":30,"c0":0,"r1":30,"c1":1} |
| ✅ | ⌘V 貼上公式並調整參照 | B33 = "=A33*2"，預期 "=A33*2" |
| ✅ | 貼上後選取貼上範圍 | [{"r0":32,"c0":0,"r1":32,"c1":1}] |
| ✅ | 貼到倍數大小的範圍會重複貼上 | B36 = "=A36*2"，預期 "=A36*2" |
| ✅ | Esc 結束虛線框 |  |
| ✅ | 外部 TSV 貼上：文字 | A38 = "名稱"，預期 "名稱" |
| ✅ | 外部 TSV 貼上：公式 | B39 = "=1+2"，預期 "=1+2" |
| ✅ | 剪下貼上移動儲存格 | D33 = "1"，預期 "1" |
| ✅ | 剪下後原位置清空 | A33 = ""，預期 "" |
| ✅ | 參照跟著移動 | B33 = "=D33*2"，預期 "=D33*2" |
| ✅ | 復原剪下貼上 | A33 = "1"，預期 "1" |
| ✅ | 選擇性貼上：值＋轉置（第一格） | F31 = "1"，預期 "1" |
| ✅ | 選擇性貼上：值＋轉置（公式變成值） | F32 = "2"，預期 "2" |
| ✅ | 填滿控點：等差數列 | A45 = "11"，預期 "11" |
| ✅ | 填滿控點：星期清單 | B43 = "星期四"，預期 "星期四" |
| ✅ | 填滿控點：公式調整參照 | C43 = "=A43*10"，預期 "=A43*10" |
| ✅ | ⌘D 向下複製 | D42 = "x"，預期 "x" |
| ✅ | 插入前的加總 | 36 |
| ✅ | ⇧⌘= 插入列：範圍擴大 | C31 = "=SUM(A40:A46)"，預期 "=SUM(A40:A46)" |
| ✅ | 插入列：下方儲存格下移 | A46 = "11"，預期 "11" |
| ✅ | ⌘- 刪除列：範圍還原 | C31 = "=SUM(A40:A45)"，預期 "=SUM(A40:A45)" |
| ✅ | 插入欄：公式跟著右移 | D31 = "=SUM(B40:B45)"，預期 "=SUM(B40:B45)" |
| ✅ | 復原插入欄 | C31 = "=SUM(A40:A45)"，預期 "=SUM(A40:A45)" |
| ✅ | ⌃0 隱藏欄 | [{"i":1,"px":86},{"i":4,"px":120,"hidden":true}] |
| ✅ | ⌃⇧0 取消隱藏欄 | [{"i":4,"px":120},{"i":1,"px":86}] |
| ✅ | ⌃9 隱藏列 | [{"i":21,"px":41},{"i":0,"px":41},{"i":33,"px":20,"hidden":true},{"i":14,"px":23}] |
| ✅ | 尋找 | {"sheet":0,"row":39,"col":1,"v":""} |
| ✅ | 全部取代 | {"value":4,"error":null} |
| ✅ | 取代結果 | B42 = "週三"，預期 "週三" |
| ✅ | 遞減排序（自動判斷標題列） | 甲 乙 |
| ✅ | ⇧⌘F 開啟篩選 | {"r0":50,"c0":7,"r1":53,"c1":8} |
| ✅ | 篩選清單 | [{"text":"丙","shown":true},{"text":"乙","shown":true},{"text":"甲","shown":true}] |
| ✅ | 篩選隱藏不符合的列 | [52,53] |
| ✅ | 篩選欄顯示漏斗 | [7] |
| ✅ | 凍結窗格 | 1,1 |
| ✅ | xlsx 存讀：凍結窗格 | 1,1 |
| ✅ | xlsx 存讀：篩選範圍與隱藏列 | {"r0":50,"c0":7,"r1":53,"c1":8} |
| ✅ | xlsx 存讀：插入刪除後的公式 | C31 = "=SUM(A40:A45)"，預期 "=SUM(A40:A45)" |
| ✅ | 名稱方塊定義名稱 | [{"name":"數量","refersTo":"=工作表1!$A$61:$A$63","scope":-1,"comment":""}] |
| ✅ | 自動完成列出名稱 | ["數量"] |
| ✅ | 使用名稱的公式 | A64 = "=SUM(數量)"，預期 "=SUM(數量)" |
| ✅ | 名稱公式的結果 | {"v":"6","style":{"id":0,"font":{"name":"Calibri","size":11},"left":{},"right":{},"top":{},"bottom":{}},"bar":null} |
| ✅ | 名稱方塊跳到名稱的範圍 | [{"r0":60,"c0":0,"r1":62,"c1":0}] |
| ✅ | 名稱方塊跳到位址 | {"row":4,"col":2} |
| ✅ | 中文字型會套用（宋體、楷體與蘋方字寬不同） | 157.75 150.63 151.46 157.83 157.83 |
| ✅ | Windows 中文字型對應到 macOS 字型 | 157.75 150.63 151.46 157.83 157.83 |
| ✅ | 插入註解 | [{"row":60,"col":1,"author":"Andrew Lu","text":"記得核對"}] |
| ✅ | 清單選項 | ["是","否"] |
| ✅ | 資料驗證拒絕清單外的值 | {"suggestions":[],"hint":null,"error":"請選擇是或否","highlights":[]} |
| ✅ | 資料驗證接受清單內的值 | D62 = "是"，預期 "是" |
| ✅ | 條件式格式套用 | {"hit":{"v":"2","style":{"id":23,"font":{"name":"Calibri","size":11,"color":"9C0006"},"fill":"FFC7CE","left":{},"right":{},"top":{},"bottom":{}},"bar":null},"miss":{"v":"1","style":{"id":0,"font":{"name":"Calibri","size":11},"left":{},"right":{},"top":{},"bottom":{}},"bar":null}} |
| ✅ | 資料橫條 | {"v":"10","style":{"id":0,"font":{"name":"Calibri","size":11},"left":{},"right":{},"top":{},"bottom":{}},"bar":0.3333333333333333} |
| ✅ | 值改變後條件式格式更新 | {"v":"5","style":{"id":23,"font":{"name":"Calibri","size":11,"color":"9C0006"},"fill":"FFC7CE","left":{},"right":{},"top":{},"bottom":{}},"bar":null} |
| ✅ | 建立表格 | {"value":"表格1","error":null} |
| ✅ | 表格往右擴充並計算 [@欄] | {"v":"20","style":{"id":25,"font":{"name":"Calibri","size":11},"fill":"D9E1F2","left":{},"right":{},"top":{},"bottom":{}},"bar":null} |
| ✅ | 表格往下擴充 | {"name":"表格1","range":{"r0":60,"c0":7,"r1":63,"c1":9},"totals":false,"style":"TableStyleMedium2","bands":true} |
| ✅ | 結構化參照加總 | {"v":"30","style":{"id":0,"font":{"name":"Calibri","size":11},"left":{},"right":{},"top":{},"bottom":{}},"bar":null} |
| ✅ | 表格標題列樣式 | {"v":"品項","style":{"id":24,"font":{"name":"Calibri","size":11,"bold":true,"color":"FFFFFF"},"fill":"4472C4","left":{},"right":{},"top":{},"bottom":{}},"bar":null} |
| ✅ | xlsx 存讀：名稱 | [{"name":"數量","refersTo":"=工作表1!$A$61:$A$63","scope":-1,"comment":""}] |
| ✅ | xlsx 存讀：註解 | [{"row":60,"col":1,"author":"Andrew Lu","text":"記得核對"}] |
| ✅ | xlsx 存讀：資料驗證 |  |
| ✅ | xlsx 存讀：條件式格式 | {"v":"2","style":{"id":23,"font":{"name":"Calibri","size":11,"color":"9C0006"},"fill":"FFC7CE","left":{},"right":{},"top":{},"bottom":{}},"bar":null} |
| ✅ | xlsx 存讀：表格與結構化參照 | [{"name":"表格1","range":{"r0":60,"c0":7,"r1":64,"c1":9},"totals":false,"style":"TableStyleMedium2","bands":true}] |
| ✅ | 從資料建立圖表 | {"value":{"type":"column","title":"","border":"D9D9D9","series":[{"name":"=工作表1!$B$81","categories":"=工作表1!$A$82:$A$84","values":"=工作表1!$B$82:$B$84"},{"name":"=工作表1!$C$81","categories":"=工作表1!$A$82:$A$84","values":"=工作表1!$C$82:$C$84"}],"legend":"right","xTitle":"","yTitle":"","stacked":false},"error":null} |
| ✅ | 圖表資料 | {"categories":["一月","二月","三月"],"series":[{"name":"收入","values":[100,150,120],"x":[]},{"name":"支出","values":[80,90,70],"x":[]}]} |
| ✅ | 圖表跟著資料更新 |  |
| ✅ | 物件層顯示圖表 | [{"id":1,"kind":"chart","row":80,"col":4,"dx":0,"dy":0,"w":480,"h":288,"chart":{"type":"column","title":"收支","border":"D9D9D9","series":[{"name":"=工作表1!$B$81","categories":"=工作表1!$A$82:$A$84","values":"=工作表1!$B$82:$B$84"},{"name":"=工作表1!$C$81","categories":"=工作表1!$A$82:$A$84","values":"=工作表1!$C$82:$C$84"}],"legend":"right","xTitle":"","yTitle":"","stacked":false},"imageKey":""}] |
| ✅ | 圖表畫成圖片（列印用） | 33742 bytes |
| ✅ | 移動並調整圖表 | {"id":1,"kind":"chart","row":81,"col":5,"dx":10,"dy":4,"w":400,"h":240,"chart":{"type":"column","title":"收支","border":"D9D9D9","series":[{"name":"=工作表1!$B$81","categories":"=工作表1!$A$82:$A$84","values":"=工作表1!$B$82:$B$84"},{"name":"=工作表1!$C$81","categories":"=工作表1!$A$82:$A$84","values":"=工作表1!$C$82:$C$84"}],"legend":"right","xTitle":"","yTitle":"","stacked":false},"imageKey":""} |
| ✅ | 插入列時圖表下移、參照調整 | [{"name":"=工作表1!$B$83","categories":"=工作表1!$A$84:$A$86","values":"=工作表1!$B$84:$B$86"},{"name":"=工作表1!$C$83","categories":"=工作表1!$A$84:$A$86","values":"=工作表1!$C$84:$C$86"}] |
| ✅ | 插入列後圖表資料不變 |  |
| ✅ | 預設版面設定 | {"paper":"A4","landscape":false,"top":0.75,"bottom":0.75,"left":0.7,"right":0.7,"header":0.3,"footer":0.3,"scale":100,"fitWide":0,"fitTall":0,"printArea":null,"gridlines":false,"headings":false,"centerH":false,"centerV":false,"headerText":"","footerText":"第 &P 頁，共 &N 頁"} |
| ✅ | 產生預覽 PDF | {"value":{"url":"/print/dc418d16d4f51348.pdf","pages":1},"error":null} |
| ✅ | 預覽 PDF 可以載入 | 200 application/pdf %PDF- |
| ✅ | xlsx 存讀：圖表 | {"id":1,"kind":"chart","row":83,"col":5,"dx":10,"dy":4,"w":400,"h":240,"chart":{"valueAxis":{"fontSize":9,"fontName":"+mn-lt","format":"General","majorUnit":0,"lineColor":"D9D9D9","lineHidden":false,"hidden":false,"majorTick":"none","gridLines":false,"hideLabels":false},"categoryAxis":{"fontSize":9,"fontName":"+mn-lt","format":"General","majorUnit":0,"lineColor":"D9D9D9","lineHidden":false,"hidden":false,"majorTick":"none","gridLines":false,"hideLabels":false},"type":"column","title":"收支","border":"D9D9D9","series":[{"labels":{"sourceConfig":"{\"format\":\"\",\"showValue\":false,\"showPercent\":false,\"fontSize\":0,\"color\":\"\",\"points\":{}}","format":"","showValue":false,"showPercent":false,"fontSize":0,"color":"","points":{},"rawXML":"<dLbls xmlns=\"http://schemas.openxmlformats.org/drawingml/2006/chart\" xmlns:a=\"http://schemas.openxmlformats.org/drawingml/2006/main\"><spPr></spPr><txPr><a:bodyPr anchorCtr=\"false\" rot=\"0\" spcFirstLastPara=\"false\"></a:bodyPr><a:p><a:pPr><a:defRPr b=\"false\" baseline=\"0\" i=\"false\" kern=\"0\" spc=\"0\"></a:defRPr></a:pPr></a:p></txPr><showLegendKey val=\"0\"></showLegendKey><showVal val=\"0\"></showVal><showCatName val=\"0\"></showCatName><showSerName val=\"0\"></showSerName><showPercent val=\"0\"></showPercent><showBubbleSize val=\"0\"></showBubbleSize><showLeaderLines val=\"0\"></showLeaderLines></dLbls>"},"name":"=工作表1!$B$83","categories":"=工作表1!$A$84:$A$86","values":"=工作表1!$B$84:$B$86"},{"labels":{"sourceConfig":"{\"format\":\"\",\"showValue\":false,\"showPercent\":false,\"fontSize\":0,\"color\":\"\",\"points\":{}}","format":"","showValue":false,"showPercent":false,"fontSize":0,"color":"","points":{},"rawXML":"<dLbls xmlns=\"http://schemas.openxmlformats.org/drawingml/2006/chart\" xmlns:a=\"http://schemas.openxmlformats.org/drawingml/2006/main\"><spPr></spPr><txPr><a:bodyPr anchorCtr=\"false\" rot=\"0\" spcFirstLastPara=\"false\"></a:bodyPr><a:p><a:pPr><a:defRPr b=\"false\" baseline=\"0\" i=\"false\" kern=\"0\" spc=\"0\"></a:defRPr></a:pPr></a:p></txPr><showLegendKey val=\"0\"></showLegendKey><showVal val=\"0\"></showVal><showCatName val=\"0\"></showCatName><showSerName val=\"0\"></showSerName><showPercent val=\"0\"></showPercent><showBubbleSize val=\"0\"></showBubbleSize><showLeaderLines val=\"0\"></showLeaderLines></dLbls>"},"name":"=工作表1!$C$83","categories":"=工作表1!$A$84:$A$86","values":"=工作表1!$C$84:$C$86"}],"legend":"right","xTitle":"","yTitle":"","stacked":false},"imageKey":""} |
| ✅ | xlsx 存讀：版面設定 | {"paper":"A4","landscape":true,"top":0.75,"bottom":0.75,"left":0.7,"right":0.7,"header":0.3,"footer":0.3,"scale":100,"fitWide":0,"fitTall":0,"printArea":{"r0":82,"c0":0,"r1":95,"c1":12},"gridlines":false,"headings":false,"centerH":false,"centerV":false,"headerText":"&A","footerText":"第 &P 頁，共 &N 頁"} |
| ✅ | 偏好設定：預設值 | {"appearance":"system","language":"system","autoRecoverMinutes":2,"enterMove":"down","checkUpdates":true,"lang":"zh-TW"} |
| ✅ | 外觀：強制深色 | theme=dark --grid-bg=#fff |
| ✅ | 外觀：強制淺色 | theme=light --grid-bg=#fff |
| ✅ | 外觀：跟隨系統 | undefined |
| ✅ | Enter 向右：提交 | A101 = "右"，預期 "右" |
| ✅ | Enter 向右：移到右邊 | 作用中 B101 |
| ✅ | ⇧Enter 反方向 | 作用中 A101 |
| ✅ | Enter 不移動 | 作用中 A101 |
| ✅ | 還原 Enter 向下 | 作用中 A102 |
| ✅ | VoiceOver：播報儲存格與值 | "A101 右" |
| ✅ | VoiceOver：空白儲存格 | "B101 空白" |
| ✅ | VoiceOver：選取範圍 | "已選取 B101:C101，B101 空白" |
| ✅ | VoiceOver：live region | 已選取 B101:C101，B101 空白 |
| ✅ | 複製格式：按下刷子待命 |  |
| ✅ | 複製格式：擷取來源不新增復原紀錄 |  |
| ✅ | 複製格式：拖曳套用完整格式 | {"id":30,"font":{"name":"Calibri","size":11,"bold":true,"color":"C00000"},"fill":"FFFF00","left":{"style":"thin"},"right":{"style":"thin"},"top":{"style":"thin"},"bottom":{"style":"thin"},"alignH":"center","numFmt":"0.00%"} |
| ✅ | 複製格式：保留目標公式 | =2+3 |
| ✅ | 複製格式：單次套用後結束 |  |
| ✅ | 複製格式：一次復原整個範圍 |  |
| ✅ | 複製格式：重做 |  |
| ✅ | 複製格式：雙擊可連續套用 |  |
| ✅ | 複製格式：Escape 取消 |  |
| ✅ | 複製格式：多格來源重複圖樣 |  |
| ✅ | 複製格式：跨工作表套用 |  |
| ✅ | 開檔套用 XLSX 視窗大小 | {"height":480,"width":740} |
| ✅ | 螢幕可用範圍使用邏輯點 | {"area":{"X":0,"Y":33,"Width":1512,"Height":886},"width":1512,"height":886} |
| ✅ | 開檔套用 XLSX 視窗位置並避開系統介面 | {"openedPosition":{"x":0,"y":33},"area":{"X":0,"Y":33,"Width":1512,"Height":886}} |
| ✅ | 縮放 10%：保留捲動錨點 | {"anchor":{"row":75,"col":1},"before":{"x":75,"y":33.24999999999994,"w":64.5,"h":15},"after":{"x":7.500000000000001,"y":3.025000000000012,"w":6.450000000000001,"h":1.5000000000000002}} |
| ✅ | 縮放 10%：格線尺寸 | {"x":66.15,"y":57.02500000000002,"w":4.800000000000001,"h":1.5000000000000002} |
| ✅ | 縮放 10%：命中儲存格 | {"row":110,"col":12} |
| ✅ | 縮放 10%：滑鼠座標選格 | {"row":110,"col":12} |
| ✅ | 縮放 10%：雙擊進入正確儲存格 | {"mode":"edit","pos":{"row":110,"col":12}} |
| ✅ | 縮放 10%：編輯框位置 | font-style: normal; font-variant-caps: normal; font-weight: normal; font-width: normal; font-size: 1.1px; line-height: 1px; font-family: Calibri, Helvetica Neue, PingFang TC, Heiti TC, -apple-system, sans-serif; font-size-adjust: none; font-kerning: auto; font-variant-alternates: normal; font-variant-ligatures: normal; font-variant-numeric: normal; font-variant-east-asian: normal; font-variant-position: normal; font-variant-emoji: normal; font-feature-settings: normal; font-optical-sizing: auto; font-variation-settings: normal; padding: 0.075px 0.225px 0px 0.3px; left: 65.15px; top: 56.025px; width: 5.8px; height: 3px; |
| ✅ | 縮放 10%：文字大小 | 1.1px |
| ✅ | 縮放 70%：保留捲動錨點 | {"anchor":{"row":75,"col":1},"before":{"x":7.500000000000001,"y":4.525000000000012,"w":6.450000000000001,"h":1.5000000000000002},"after":{"x":52.49999999999999,"y":31.674999999999834,"w":45.14999999999999,"h":10.499999999999998}} |
| ✅ | 縮放 70%：格線尺寸 | {"x":463.0499999999999,"y":399.1749999999998,"w":33.599999999999994,"h":10.499999999999998} |
| ✅ | 縮放 70%：命中儲存格 | {"row":110,"col":12} |
| ✅ | 縮放 70%：滑鼠座標選格 | {"row":110,"col":12} |
| ✅ | 縮放 70%：雙擊進入正確儲存格 | {"mode":"edit","pos":{"row":110,"col":12}} |
| ✅ | 縮放 70%：編輯框位置 | font-style: normal; font-variant-caps: normal; font-weight: normal; font-width: normal; font-size: 7.7px; line-height: 9px; font-family: Calibri, Helvetica Neue, PingFang TC, Heiti TC, -apple-system, sans-serif; font-size-adjust: none; font-kerning: auto; font-variant-alternates: normal; font-variant-ligatures: normal; font-variant-numeric: normal; font-variant-east-asian: normal; font-variant-position: normal; font-variant-emoji: normal; font-feature-settings: normal; font-optical-sizing: auto; font-variation-settings: normal; padding: 0.525px 1.575px 0px 2.1px; left: 462.05px; top: 398.175px; width: 34.6px; height: 12px; |
| ✅ | 縮放 70%：文字大小 | 7.7px |
| ✅ | 縮放 100%：保留捲動錨點 | {"anchor":{"row":75,"col":1},"before":{"x":52.49999999999999,"y":31.674999999999834,"w":45.14999999999999,"h":10.499999999999998},"after":{"x":75,"y":44.25,"w":64.5,"h":15}} |
| ✅ | 縮放 100%：格線尺寸 | {"x":661.5,"y":569.25,"w":48,"h":15} |
| ✅ | 縮放 100%：命中儲存格 | {"row":110,"col":12} |
| ✅ | 縮放 100%：滑鼠座標選格 | {"row":110,"col":12} |
| ✅ | 縮放 100%：雙擊進入正確儲存格 | {"mode":"edit","pos":{"row":110,"col":12}} |
| ✅ | 縮放 100%：編輯框位置 | font-style: normal; font-variant-caps: normal; font-weight: normal; font-width: normal; font-size: 11px; line-height: 13px; font-family: Calibri, Helvetica Neue, PingFang TC, Heiti TC, -apple-system, sans-serif; font-size-adjust: none; font-kerning: auto; font-variant-alternates: normal; font-variant-ligatures: normal; font-variant-numeric: normal; font-variant-east-asian: normal; font-variant-position: normal; font-variant-emoji: normal; font-feature-settings: normal; font-optical-sizing: auto; font-variation-settings: normal; padding: 0.75px 2.25px 0px 3px; left: 660.5px; top: 568.25px; width: 49px; height: 16px; |
| ✅ | 縮放 100%：文字大小 | 11px |
| ✅ | 縮放 125%：保留捲動錨點 | {"anchor":{"row":75,"col":1},"before":{"x":75,"y":44.25,"w":64.5,"h":15},"after":{"x":93.75,"y":54.56250000000004,"w":80.625,"h":18.75}} |
| ✅ | 縮放 125%：格線尺寸 | {"x":826.875,"y":569.8125,"w":60,"h":18.75} |
| ✅ | 縮放 125%：命中儲存格 | {"row":110,"col":12} |
| ✅ | 縮放 125%：滑鼠座標選格 | {"row":110,"col":12} |
| ✅ | 縮放 125%：雙擊進入正確儲存格 | {"mode":"edit","pos":{"row":110,"col":12}} |
| ✅ | 縮放 125%：編輯框位置 | font-style: normal; font-variant-caps: normal; font-weight: normal; font-width: normal; font-size: 13.75px; line-height: 17px; font-family: Calibri, Helvetica Neue, PingFang TC, Heiti TC, -apple-system, sans-serif; font-size-adjust: none; font-kerning: auto; font-variant-alternates: normal; font-variant-ligatures: normal; font-variant-numeric: normal; font-variant-east-asian: normal; font-variant-position: normal; font-variant-emoji: normal; font-feature-settings: normal; font-optical-sizing: auto; font-variation-settings: normal; padding: 0.9375px 2.8125px 0px 3.75px; left: 825.875px; top: 568.25px; width: 61px; height: 20px; |
| ✅ | 縮放 125%：文字大小 | 13.75px |
| ✅ | 縮放 200%：保留捲動錨點 | {"anchor":{"row":82,"col":1},"before":{"x":93.75,"y":44.25000000000004,"w":80.625,"h":18.75},"after":{"x":150,"y":69.88297820637251,"w":129,"h":30}} |
| ✅ | 縮放 200%：格線尺寸 | {"x":1185,"y":558.607437955784,"w":96,"h":30} |
| ✅ | 縮放 200%：命中儲存格 | {"row":110,"col":12} |
| ✅ | 縮放 200%：滑鼠座標選格 | {"row":110,"col":12} |
| ✅ | 縮放 200%：雙擊進入正確儲存格 | {"mode":"edit","pos":{"row":110,"col":12}} |
| ✅ | 縮放 200%：編輯框位置 | font-style: normal; font-variant-caps: normal; font-weight: normal; font-width: normal; font-size: 22px; line-height: 27px; font-family: Calibri, Helvetica Neue, PingFang TC, Heiti TC, -apple-system, sans-serif; font-size-adjust: none; font-kerning: auto; font-variant-alternates: normal; font-variant-ligatures: normal; font-variant-numeric: normal; font-variant-east-asian: normal; font-variant-position: normal; font-variant-emoji: normal; font-feature-settings: normal; font-optical-sizing: auto; font-variation-settings: normal; padding: 1.5px 4.5px 0px 6px; left: 1184px; top: 557px; width: 97px; height: 31px; |
| ✅ | 縮放 200%：文字大小 | 22px |
| ✅ | 縮放 400%：保留捲動錨點 | {"anchor":{"row":94,"col":2},"before":{"x":141,"y":78,"w":96,"h":30},"after":{"x":282,"y":155.06568908750637,"w":192,"h":60}} |
| ✅ | 縮放 400%：格線尺寸 | {"x":1089,"y":529.9505147300888,"w":192,"h":60} |
| ✅ | 縮放 400%：命中儲存格 | {"row":110,"col":12} |
| ✅ | 縮放 400%：滑鼠座標選格 | {"row":110,"col":12} |
| ✅ | 縮放 400%：雙擊進入正確儲存格 | {"mode":"edit","pos":{"row":110,"col":12}} |
| ✅ | 縮放 400%：編輯框位置 | font-style: normal; font-variant-caps: normal; font-weight: normal; font-width: normal; font-size: 44px; line-height: 54px; font-family: Calibri, Helvetica Neue, PingFang TC, Heiti TC, -apple-system, sans-serif; font-size-adjust: none; font-kerning: auto; font-variant-alternates: normal; font-variant-ligatures: normal; font-variant-numeric: normal; font-variant-east-asian: normal; font-variant-position: normal; font-variant-emoji: normal; font-feature-settings: normal; font-optical-sizing: auto; font-variation-settings: normal; padding: 3px 9px 0px 12px; left: 1088px; top: 527px; width: 193px; height: 61px; |
| ✅ | 縮放 400%：文字大小 | 44px |
| ✅ | 縮放不修改文件或復原紀錄 | {"name":"selftest-v07.xlsx","path":"/tmp/ww-format22-desktop/selftest-v07.xlsx","dirty":true,"canUndo":true,"canRedo":false,"undoLabel":"凍結窗格","redoLabel":"","circular":"","sheets":["工作表1","報表","工作表3"],"tabColors":["","",""],"names":["數量"]} |
| ✅ | 點擊百分比開啟縮放視窗 |  |
| ✅ | 選擇預設比例尚不套用 | 400 |
| ✅ | 確定套用預設比例 | 75 |
| ✅ | 取消保留原縮放比例 | 75 |
| ✅ | 關閉視窗恢復格線焦點 | DIV |
| ✅ | 輸入百分比 | 70% |
| ✅ | 無效百分比保留原值 | 70% |
| ✅ | Escape 取消百分比 | 70% |
| ✅ | 縮放上限 | 400% |
| ✅ | 縮放下限 | 10% |
| ✅ | 加號增加 10% | 110% |
| ✅ | 減號減少 10% | 100% |
| ✅ | 選取範圍最適化容納全部選取範圍 | {"first":{"x":126.75999999999998,"y":80.02000000000014,"w":80.64,"h":25.2},"last":{"x":1013.8,"y":558.8200000000002,"w":80.64,"h":25.2},"zoom":"168"} |
| ✅ | 圖表隨比例縮放 | width: 600px; height: 360px; |
| ✅ | 重新命名保留縮放 | 70% |
| ✅ | 新工作表預設 100% | 100% |
| ✅ | 切回工作表保留 70% | 70% |
| ✅ | 各工作表獨立記住比例 | 200% |
| ✅ | 工作表排序保留縮放 | 70% |
| ✅ | 重新開檔恢復 100% | 100% |
| ✅ | Ribbon：六個分頁 | 6 |
| ✅ | Ribbon：常用 顯示正確面板 | active=0 |
| ✅ | Ribbon：插入 顯示正確面板 | active=1 |
| ✅ | Ribbon：頁面配置 顯示正確面板 | active=2 |
| ✅ | Ribbon：公式 顯示正確面板 | active=3 |
| ✅ | Ribbon：資料 顯示正確面板 | active=4 |
| ✅ | Ribbon：檢視 顯示正確面板 | active=5 |
| ✅ | Ribbon：方向鍵切換並移動焦點 | ribbon-tab-1 |
| ✅ | Ribbon：Home 返回常用 | Home |
| ✅ | Ribbon：窄視窗維持高度 | 123 / 123 |
| ✅ | Ribbon：群組維持同一列並可水平捲動 | 620 / 1099 |
| ✅ | Ribbon：所有控制項保持在固定高度內 |  |
| ✅ | Ribbon：溢出時顯示向右按鈕 | 620px |
| ✅ | Ribbon：向右按鈕位於工具列上方且可點選 | <button class="scroll-arrow right svelte-kny9im" aria-label="向右" title="向右">›</button> |
| ✅ | Ribbon：向右按鈕捲動並顯示向左按鈕 | 434 |
| ✅ | Ribbon：向左按鈕位於工具列上方且可點選 | <button class="scroll-arrow left svelte-kny9im" aria-label="向左" title="向左">‹</button> |
| ✅ | Ribbon：向左按鈕返回起點 | 0 |
| ✅ | Ribbon：滑鼠滾輪水平捲動 | 100 |
| ✅ | Ribbon：捲動停止後雙向按鈕保持可見 | 100 |
| ✅ | Ribbon：窄視窗色盤未被裁切且可點選 | {"x":174,"y":125,"width":225,"height":230,"top":125,"right":399,"bottom":355,"left":174} |
| ✅ | Ribbon：捲動時關閉浮動選單 | scroll |
| ✅ | Ribbon：Escape 關閉浮動選單 | Escape |
| ✅ | Ribbon：最右側功能可捲入且隱藏向右按鈕 | 479 |
| ✅ | Ribbon：清除選單未被裁切且可點選 | {"x":430,"y":122,"width":120,"height":88,"top":122,"right":550,"bottom":210,"left":430} |
| ✅ | Ribbon：插入 窄視窗維持高度 | 123 |
| ✅ | Ribbon：頁面配置 窄視窗維持高度 | 123 |
| ✅ | Ribbon：公式 窄視窗維持高度 | 123 |
| ✅ | Ribbon：資料 窄視窗維持高度 | 123 |
| ✅ | Ribbon：檢視 窄視窗維持高度 | 123 |
| ✅ | Ribbon：格式按鈕保留工作表焦點 | DIV |
| ✅ | Ribbon：色彩選單可以開啟 | font color |
| ✅ | 儲存格選單：右下角完整位於視窗內 | {"x":537,"y":91,"width":200,"height":354,"top":91,"right":737,"bottom":445,"left":537} |
| ✅ | 儲存格選單：不受工作表容器裁切 | BODY |
| ✅ | 儲存格選單：捲動選單不會關閉 | 234 |
| ✅ | 儲存格選單：最末項目可捲動至可點選位置 | <button role="menuitem" class="svelte-1v8btbv"><span>儲存格格式…</span> <kbd class="svelte-1v8btbv">⌘1</kbd><!----></button> |
| ✅ | 儲存格選單：Esc 移除浮層 | Escape |
| ✅ | 儲存格選單：上方開啟也保留視窗內邊距 | {"x":60,"y":91,"width":200,"height":354,"top":91,"right":260,"bottom":445,"left":60} |
| ✅ | 儲存格選單：視窗變更時關閉浮層 | resize |
| ✅ | 儲存格選單：工作表滾輪操作時關閉浮層 | wheel |
| ✅ | 工作表選單：多張時允許刪除 | 3 sheets |
| ✅ | 工作表選單：刪除右鍵選中的工作表 | 2 |
| ✅ | 工作表選單：操作後關閉 | delete |
| ✅ | 工作表選單：左移 | 1,0 |
| ✅ | 工作表選單：右移 | 1,2 |
| ✅ | 工作表選單：最後一張禁止刪除 | 1 sheet |
| ✅ | 工作表選單：最後一張可見工作表禁止刪除 | 1 visible sheet |
| ✅ | 工作表操作：點加號新增並顯示分頁 | ["工作表1","工作表4","報表","工作表3"] |
| ✅ | 工作表操作：右鍵左移同步到 Go | ["工作表4","工作表1","報表","工作表3"] |
| ✅ | 工作表操作：右鍵右移同步到 Go | ["工作表1","工作表4","報表","工作表3"] |
| ✅ | 工作表操作：右鍵刪除同步到 Go | ["工作表1","報表","工作表3"] |
| ✅ | 工作表操作：刪除後分頁消失 | 3 |
| ✅ | 測試 bindings 安全：拒絕負的事件數 | RuntimeError: invalid event count or blocks per event |
| ✅ | 測試 bindings 安全：拒絕過大的區塊數 | RuntimeError: invalid event count or blocks per event |
| ✅ | 測試 bindings 安全：拒絕過大的事件總量 | RuntimeError: test event block count exceeds limit |
| ✅ | 測試 bindings 安全：拒絕過大的公式鏈 | RuntimeError: test formula chain must contain 1..100000 cells |
| ✅ | 測試 bindings 安全：合法探測仍可使用 | 1794034790596 |
| ✅ | 測試 bindings 安全：拒絕後可繼續執行 | 0.003 |
| ✅ | 公式安全：REPT 溢位回傳錯誤 | #VALUE! |
| ✅ | 公式安全：前端收到錯誤結果 | {"text":"#VALUE!","numeric":false,"center":true,"style":0,"color":null,"bar":null} |
| ✅ | 公式安全：錯誤後可查詢文件狀態 | {"name":"selftest-zoom.xlsx","path":"/tmp/ww-format22-desktop/selftest-zoom.xlsx","dirty":true,"canUndo":true,"canRedo":false,"undoLabel":"輸入","redoLabel":"","circular":"","sheets":["工作表1","報表","工作表3"],"tabColors":["","",""],"names":["數量"]} |
| ✅ | 公式安全：錯誤公式可復原 |  |
| ✅ | 公式安全：錯誤公式可重做 | #VALUE! |
| ✅ | 公式安全：錯誤後可繼續編輯 | 3 |
| ✅ | 公式安全：xlsx 儲存重開保留錯誤與其他資料 | #VALUE! / 3 |
| ✅ | 公式安全：xlsx 重開後文件狀態正常 | {"name":"selftest-formula-errors.xlsx","path":"/tmp/ww-format22-desktop/selftest-formula-errors.xlsx","dirty":false,"canUndo":false,"canRedo":false,"undoLabel":"","redoLabel":"","circular":"","sheets":["工作表1","報表","工作表3"],"tabColors":["","",""],"names":["數量"]} |
| ✅ | 公式安全：wwx 儲存重開保留錯誤與其他資料 | #VALUE! / 3 |
| ✅ | 公式安全：wwx 重開後文件狀態正常 | {"name":"selftest-formula-errors.wwx","path":"/tmp/ww-format22-desktop/selftest-formula-errors.wwx","dirty":false,"canUndo":false,"canRedo":false,"undoLabel":"","redoLabel":"","circular":"","sheets":["工作表1","報表","工作表3"],"tabColors":["","",""],"names":["數量"]} |

## 儲存格格式視窗

- 結果：14 / 14 通過

| | 項目 |
| --- | --- |
| ✅ | ⌘1 開啟五分頁共用視窗 |
| ✅ | 色盤選色後焦點回到色彩按鈕 |
| ✅ | 色盤選色不會提前提交 |
| ✅ | 切換分頁保留格式碼與 Go 預覽 |
| ✅ | 草稿尚未修改文件 |
| ✅ | 確定套用跨分頁草稿 |
| ✅ | 未修改直接確定不新增復原紀錄 |
| ✅ | 無效字級跨分頁仍阻止提交 |
| ✅ | 取消保留文件原格式 |
| ✅ | Esc 關閉視窗且不修改文件 |
| ✅ | 右鍵開啟同一個五分頁視窗 |
| ✅ | 方向鍵切換分頁並移動焦點 |
| ✅ | Tab 焦點循環留在視窗內 |
| ✅ | 小視窗內容可捲動且確定按鈕留在畫面內 |

## 框線編輯與預覽

- 結果：18 / 18 通過

| | 項目 |
| --- | --- |
| ✅ | 開啟視窗預覽既有框線 |
| ✅ | 框線分頁提供十三種線型 |
| ✅ | 雙線外框預覽使用選定色彩 |
| ✅ | 內框可使用不同線型與色彩 |
| ✅ | 預覽編輯尚未寫入文件 |
| ✅ | 點選預覽可清除指定邊線 |
| ✅ | 第三狀態保留原框線並預覽原色 |
| ✅ | 預覽可用空白鍵切換而不提交 |
| ✅ | 多格範圍套用雙線外框與點線內框 |
| ✅ | 相鄰儲存格共用邊界同步 |
| ✅ | 框線設定一次復原 |
| ✅ | 框線設定一次重做 |
| ✅ | 清除左框線也清除鄰格右框線 |
| ✅ | 指定清除保留其餘邊線 |
| ✅ | 取消框線草稿不修改文件 |
| ✅ | 單一合併儲存格停用內框控制 |
| ✅ | 合併儲存格左上角保留完整外框 |
| ✅ | 合併儲存格底部與鄰格同步 |

## 單色填滿編輯

- 結果：20 / 20 通過

| | 項目 |
| --- | --- |
| ✅ | 白色填滿預覽不透明 |
| ✅ | 選色只修改草稿 |
| ✅ | 跨分頁保留填滿草稿 |
| ✅ | 取消白色草稿保留原填色 |
| ✅ | 白色填滿正確套用到多格 |
| ✅ | 填滿一次復原 |
| ✅ | 填滿一次重做 |
| ✅ | 無填滿預覽透明且有明確標示 |
| ✅ | Esc 不套用無填滿草稿 |
| ✅ | 移除填滿保留字型與數值格式 |
| ✅ | 條件格式覆蓋顯示並保留直接填色 |
| ✅ | 格式視窗讀取底層填色 |
| ✅ | 可套用與顯示色相同的白色直接填滿 |
| ✅ | 直接白色優先於表格交錯列色 |
| ✅ | 清除直接填色後恢復表格列色 |
| ✅ | 可輸出實際填滿 PDF |
| ✅ | xlsx 重開區分無填滿與白色 |
| ✅ | xlsx 重開保留表格與直接格式優先順序 |
| ✅ | wwx 重開區分無填滿與白色 |
| ✅ | wwx 重開保留表格與直接格式優先順序 |

## 多選混合格式與一次復原

- 結果：24 / 24 通過

| | 項目 |
| --- | --- |
| ✅ | 數值格式混合時不顯示作用中儲存格的格式碼 |
| ✅ | 混合字型與字級顯示混合狀態 |
| ✅ | 混合粗體顯示中間狀態 |
| ✅ | 混合框線保留未修改狀態 |
| ✅ | 混合填滿不顯示單格代表色 |
| ✅ | 未修改確定不增加修改或復原紀錄 |
| ✅ | 重設草稿恢復混合填滿 |
| ✅ | 重設後確定不修改文件 |
| ✅ | 僅改填滿保留混合字型與數值格式 |
| ✅ | 僅改填滿保留各格原框線 |
| ✅ | 填滿一次復原保留原先混合值 |
| ✅ | 共同值修改後改回原值不產生紀錄 |
| ✅ | 作用中儲存格已無填滿仍可清除其他格填色 |
| ✅ | 跨分頁同時套用字型字級粗體換行與填滿 |
| ✅ | 大字級與換行產生自動列高 |
| ✅ | 一次復原同時還原格式與自動列高 |
| ✅ | 一次重做同時還原格式與自動列高 |
| ✅ | 跨分頁格式不覆寫手動列高 |
| ✅ | 手動列高在復原後仍保留 |
| ✅ | 不連續多選只更新所選範圍 |
| ✅ | 不連續選取一次復原 |
| ✅ | 取消混合草稿不增加修改紀錄 |
| ✅ | xlsx 重開保留混合格式與合併列高提交結果 |
| ✅ | wwx 重開保留混合格式與合併列高提交結果 |

## 格式相容性桌面驗收

- 結果：10 / 10 通過

| | 項目 |
| --- | --- |
| ✅ | 合併區內部單格開啟視窗後更新合併錨點 |
| ✅ | 合併格式一次復原保留合併與內容 |
| ✅ | 合併格式一次重做 |
| ✅ | 整欄格式套用至工作表最後一列 |
| ✅ | 整欄填滿保留繼承字型 |
| ✅ | 整欄格式一次復原 |
| ✅ | 整列格式套用至工作表最後一欄 |
| ✅ | 列欄交點遵循列格式優先 |
| ✅ | xlsx 重開保留合併、整列整欄與繼承 |
| ✅ | wwx 重開保留合併、整列整欄與繼承 |
