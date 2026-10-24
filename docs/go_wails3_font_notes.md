# Go + Wails3 在 macOS 使用中文字型的筆記

在 ww-excel 開發時踩到的坑：**選了中文字型卻沒有反應，英文字型正常**。這份筆記整理原因與做法，給之後的 Wails3（WKWebView）應用參考。

## 結論先講

1. **不要用 Windows 字型名稱**（新細明體、微軟正黑體、標楷體）：macOS 沒有這些字型，WebKit 找不到就默默退回預設字型，不會報錯。
2. **macOS 上很多中文字型「有安裝」但網頁用不到**：楷體、標楷體、儷宋、圓體、行楷等是「可下載字型」，WebKit 為了防止字型指紋辨識（fingerprinting），不讓網頁內容使用它們。WKWebView（也就是 Wails）一樣受限制。
3. **網頁直接可用的中文字型只有系統內建的幾個**：蘋方（`PingFang TC`）、宋體（`Songti TC`）、黑體（`Heiti TC`）。
4. 要用其他字型：由 Go 讀出字型檔，透過 asset server 提供給前端，再用 `FontFace` 載入。
5. **一定要寫自動檢查**：字型找不到時完全沒有錯誤訊息，只是畫面看起來「差不多」，很容易漏掉。

---

## 1. 判斷字型在 macOS 上叫什麼、放在哪裡

字型選單顯示的是本地化名稱（「楷體-繁」），CSS 和 Canvas 要用的是英文的 family 名稱（`Kaiti TC`）。用 CoreText 查最準：

```swift
// fonts.swift — 執行：swift fonts.swift
import CoreText
import Foundation
for n in ["PingFang TC", "Songti TC", "Kaiti TC", "BiauKaiTC", "LiSong Pro", "新細明體"] {
  let f = CTFontCreateWithName(n as CFString, 12, nil)
  let url = CTFontCopyAttribute(f, kCTFontURLAttribute) as? URL
  print("\(n)\t\(CTFontCopyFamilyName(f))\t\(url?.path ?? "-")")
}
```

- 回傳的 family 名稱和輸入不同（例如變成 `Helvetica`）：**這台 Mac 沒有這個字型**。
- 路徑在 `/System/Library/Fonts/...`：系統內建，網頁可以直接用。
- 路徑在 `/System/Library/AssetsV2/com_apple_MobileAsset_Font*/...`：**可下載字型，網頁用不到**。

實測（macOS 26）的結果：

| 字型（顯示名稱） | CSS family | 位置 | 網頁直接可用 |
|---|---|---|---|
| 蘋方-繁 | `PingFang TC` | 系統 | ✅ |
| 宋體-繁 | `Songti TC` | 系統 | ✅ |
| 黑體-繁 | `Heiti TC` | 系統 | ✅ |
| 楷體-繁 | `Kaiti TC` | AssetsV2 | ❌ |
| 標楷體-繁 | `BiauKaiTC`（注意沒有空格） | AssetsV2 | ❌ |
| 儷宋 Pro／儷黑 Pro | `LiSong Pro`／`LiHei Pro` | AssetsV2 | ❌ |
| 圓體、蘭亭黑、行楷、隸變、手札體、翩翩體 | `Yuanti TC`、`Lantinghei TC`、`Xingkai TC`、`Libian TC`、`Hannotate TC`、`HanziPen TC` | AssetsV2 | ❌ |
| 新細明體、微軟正黑體、標楷體（Windows） | — | 沒有安裝 | ❌ |

> 簡體中文也一樣：`PingFang SC`、`Songti SC`、`Heiti SC` 可用；`Kaiti SC`、`STFangsong` 是可下載字型。

## 2. Windows 字型名稱要對應到 macOS 字型

讀進來的 Excel、Word 檔常常用 Windows 字型。保留原名稱（存回檔案時才不會變），只在繪製時加上替代字型：

```ts
const ALIASES: Record<string, string[]> = {
  新細明體: ['LiSong Pro', 'Apple LiSung', 'Songti TC'],
  pmingliu: ['LiSong Pro', 'Apple LiSung', 'Songti TC'],
  微軟正黑體: ['PingFang TC', 'Heiti TC'],
  'microsoft jhenghei': ['PingFang TC', 'Heiti TC'],
  標楷體: ['BiauKaiTC', 'Kaiti TC'],
  'dfkai-sb': ['BiauKaiTC', 'Kaiti TC'],
  微软雅黑: ['PingFang SC', 'Heiti SC'],
  宋体: ['Songti SC'],
  黑体: ['Heiti SC'],
  楷体: ['Kaiti SC'],
};

export function fontFamilies(name: string): string[] {
  return [name, ...(ALIASES[name.trim().toLowerCase()] ?? [])];
}

// CSS / Canvas 的 font：原名稱 → 替代字型 → 一定存在的中文字型
const FALLBACK = `"PingFang TC", "Heiti TC", -apple-system, sans-serif`;
const family = fontFamilies(name).map((n) => `"${n}"`).join(', ') + ', ' + FALLBACK;
```

字型清單給使用者選時，**值存英文 family 名稱、顯示中文名稱**（`{ value: 'Kaiti TC', label: '楷體-繁' }`）。

## 3. 讓網頁用到「可下載字型」：Go 讀字型 → asset server → FontFace

### Go：用 CoreText 讀出字型表，組成 TTF

可下載字型多半是 `.ttc`（字型集合）。不必自己解析 TTC：CoreText 會直接給出某個字型的每一張表（`CTFontCopyTable`），把這些表重新組成一個 sfnt 檔就是合法的 TTF／OTF。

```go
//go:build darwin

package main

/*
#cgo LDFLAGS: -framework CoreText -framework CoreFoundation
#include <CoreText/CoreText.h>
#include <stdlib.h>

// The family must really be installed: CTFontCreateWithName silently
// returns another font (Helvetica) otherwise.
static CTFontRef openFont(const char *family) {
	CFStringRef name = CFStringCreateWithCString(NULL, family, kCFStringEncodingUTF8);
	CTFontRef font = CTFontCreateWithName(name, 12, NULL);
	CFStringRef got = CTFontCopyFamilyName(font);
	Boolean same = CFStringCompare(got, name, kCFCompareCaseInsensitive) == kCFCompareEqualTo;
	CFRelease(got);
	CFRelease(name);
	if (!same) { CFRelease(font); return NULL; }
	return font;
}
static int tableCount(CTFontRef f) {
	CFArrayRef t = CTFontCopyAvailableTables(f, kCTFontTableOptionNoOptions);
	int n = t ? (int)CFArrayGetCount(t) : 0;
	if (t) CFRelease(t);
	return n;
}
static uint32_t tableTag(CTFontRef f, int i) {
	CFArrayRef t = CTFontCopyAvailableTables(f, kCTFontTableOptionNoOptions);
	uint32_t tag = (uint32_t)(uintptr_t)CFArrayGetValueAtIndex(t, i);
	CFRelease(t);
	return tag;
}
static void *tableData(CTFontRef f, uint32_t tag, long *len) {
	CFDataRef d = CTFontCopyTable(f, tag, kCTFontTableOptionNoOptions);
	if (!d) { *len = 0; return NULL; }
	*len = CFDataGetLength(d);
	void *buf = malloc(*len);
	CFDataGetBytes(d, CFRangeMake(0, *len), buf);
	CFRelease(d);
	return buf;
}
static void closeFont(CTFontRef f) { CFRelease(f); }
*/
import "C"
```

Go 端把表依 tag 排序，寫入 sfnt 標頭和表目錄（每張表 4 bytes 對齊、附 checksum）；有 `CFF ` 表時版本用 `OTTO`，否則用 `0x00010000`。完整程式見 ww-excel 的 `fonts_darwin.go`（約 120 行）。

> cgo 會把 CoreFoundation 的參考型別（`CTFontRef`）當成 `uintptr`，所以判斷「沒有字型」要寫 `font == 0`，不是 `nil`。

### Go：透過 Wails asset server 提供字型

Wails3 的 `AssetOptions.Middleware` 可以攔截自訂路徑，不需要另開 HTTP server：

```go
app := application.New(application.Options{
	Assets: application.AssetOptions{
		Handler:    application.AssetFileServerFS(assets),
		Middleware: application.ChainMiddleware(FontHandler),
	},
})

// FontHandler serves /fonts/<family>.ttf (cached; 404 when not installed).
func FontHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/fonts/") {
			next.ServeHTTP(w, r)
			return
		}
		family, _ := url.PathUnescape(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/fonts/"), ".ttf"))
		data, err := fontData(family) // cache this: fonts are tens of MB
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "font/ttf")
		w.Write(data)
	})
}
```

### 前端：偵測字型能不能用，不能用才載入

```ts
// A font is usable when text set in it measures differently from the
// generic fallbacks (check two, so a coincidence cannot fool it).
export function usable(family: string): boolean {
  const ctx = document.createElement('canvas').getContext('2d')!;
  const w = (font: string) => ((ctx.font = font), ctx.measureText('中文永 AgW 123').width);
  const q = `"${family}"`;
  return w(`20px ${q}, monospace`) !== w('20px monospace') || w(`20px ${q}, serif`) !== w('20px serif');
}

const loading = new Map<string, Promise<boolean>>();
export function ensureFont(family: string): Promise<boolean> {
  let p = loading.get(family);
  if (!p) {
    p = (async () => {
      if (usable(family)) return true;
      const res = await fetch(`/fonts/${encodeURIComponent(family)}.ttf`);
      if (!res.ok) return false;
      const face = new FontFace(family, await res.arrayBuffer());
      await face.load();
      document.fonts.add(face); // same family name: CSS and Canvas now find it
      return true;
    })().catch(() => false);
    loading.set(family, p);
  }
  return p;
}
```

載入完成後要做兩件事，否則畫面不會更新：

- **重畫 Canvas**：Canvas 不會因為字型載入而自動重畫（DOM 文字會）。
- **清掉文字寬度快取**：之前用替代字型量出的寬度已經不對了（換行、自動欄寬都會錯）。

字型檔很大（一個中文字型通常是數 MB 到數十 MB），所以只在真的有用到這個字型時才載入。

## 4. 一定要寫自動檢查

字型出問題時**完全沒有錯誤訊息**，畫面只是「看起來也是中文」，很容易沒發現。用 Canvas 量字寬做檢查：

```ts
const widthIn = (family: string) => {
  ctx.font = `20px "${family}", "PingFang TC", sans-serif`;
  return ctx.measureText('中文 Ag 字型').width;
};
const widths = ['PingFang TC', 'Songti TC', 'Kaiti TC'].map(widthIn);
assert(new Set(widths).size === widths.length, '中文字型沒有套用：' + widths.join(' '));
```

再搭配一張實際畫面的截圖，用肉眼確認每個字型的樣子（ww-excel 的 self-test 會產生 `docs/selftest-fonts.png`）。

## 5. 其他提醒

- **PDF 輸出另外處理**：Go 產生 PDF（例如用 gopdf）時要自己嵌入字型。`/System/Library/Fonts/Supplemental/Arial Unicode.ttf` 是單一 TTF、含中文，最省事；大部分中文字型是 `.ttc`，很多 PDF 函式庫不支援，可以用上面 CoreText 組出的 TTF 代替。gopdf 只嵌入用到的字（subset），檔案不會太大。
- **系統沒有粗體的中文字型**：Canvas 會自動加粗；Go 產生 PDF 時要自己處理，例如偏移一點重畫一次。
- **IME 輸入**：輸入框（`<textarea>`）也要套用相同的 font 設定，否則組字時看到的字型和儲存格不一致。
- **不要相信字型選單看到的名稱**：選單顯示「標楷體-繁」，CSS 要寫 `BiauKaiTC`；寫錯名稱一樣不會報錯，一律用 CoreText 查證。
