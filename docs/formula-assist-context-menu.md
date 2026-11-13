# 右鍵刪列後公式候選消失

日期：2026-11-13。

## 重現與原因

新空白工作表的公式列輸入 `=P` 時會正常顯示候選公式。對儲存格按右鍵，選「刪除…」→「整列」→「確定」後，再輸入 `=P`，候選清單便消失。

真實 WebView 的修正前測試確認：刪列成功，前端仍有 19 個 P 開頭的函數資料，但 `.suggestions` 不在 DOM 中。直接呼叫刪列而不經右鍵選單則正常。

`placeContextMenu` 將 Svelte 條件區塊末端的 `<ul>` 移至 `document.body`，以避免選單被工作表裁切。關閉選單時，原區塊的 DOM 邊界已改變，導致後續浮層的掛載位置被移除；公式候選資料雖然更新，選單卻無法顯示。

## 修正

在右鍵選單外保留一個固定容器，留在 Svelte 原來的條件區塊中；只有容器內的選單移至 body。關閉選單時，Svelte 仍有正確的區塊邊界，後續公式候選與參數提示可正常顯示。

Wails 版本維持 `v3.0.0`，沒有變更公式計算或刪列資料處理。

## 回歸驗證

新增可獨立執行的真實桌面測試，避免被既有格式視窗 self-test 的選取器問題阻擋：

```bash
mkdir -p /private/tmp/ww-excel-formula-assist-results
WW_SELFTEST=1 WW_XLSX_FIXTURE=@formula-assist \
  WW_BENCH_OUT=/private/tmp/ww-excel-formula-assist-results/report.md \
  ./bin/ww-excel
```

測試覆蓋：

- 原生 contenteditable 逐字輸入 `=P` 與 `=p`。
- 替換既有公式與組字結束事件。
- `=PRODUCT(` 的參數提示。
- 直接刪列後的公式候選。
- 儲存格右鍵刪除、選整列並確定；檢查下一列資料確實上移。
- 刪列後候選清單的 DOM、視窗內位置與實際命中測試。
- 刪列後使用方向鍵與 Tab 選擇函數，以及參數提示。

修正前可重現清單 DOM 消失；最終 production `bin/ww-excel` 的專用桌面回歸 26／26 通過，包含清單顯示、命中測試、方向鍵／Tab 與參數提示。前端型別檢查與既有 294 項單元測試通過，App 與 DMG 已重新打包並通過 ad-hoc 簽章檢查。
