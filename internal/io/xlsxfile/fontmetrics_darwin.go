//go:build darwin && cgo

package xlsxfile

/*
#cgo LDFLAGS: -framework CoreText -framework CoreFoundation
#include <CoreText/CoreText.h>
#include <stdlib.h>
static double digitAdvance(const char *family, const char *file, double size, int bold, int italic) {
 CTFontRef font = NULL;
 CFStringRef name = CFStringCreateWithCString(NULL, family, kCFStringEncodingUTF8);
 if (file != NULL) {
  CFStringRef path = CFStringCreateWithCString(NULL,file,kCFStringEncodingUTF8);
  CFURLRef url = CFURLCreateWithFileSystemPath(NULL,path,kCFURLPOSIXPathStyle,false);
  CFArrayRef descriptors = CTFontManagerCreateFontDescriptorsFromURL(url);
  if (descriptors != NULL && CFArrayGetCount(descriptors)>0) font=CTFontCreateWithFontDescriptor((CTFontDescriptorRef)CFArrayGetValueAtIndex(descriptors,0),size,NULL);
  if (descriptors != NULL) CFRelease(descriptors);
  CFRelease(url);CFRelease(path);
 }
 if (font == NULL) font=CTFontCreateWithName(name,size,NULL);
 CFRelease(name);
 CTFontSymbolicTraits traits=(bold?kCTFontBoldTrait:0)|(italic?kCTFontItalicTrait:0);
 CTFontRef styled=CTFontCreateCopyWithSymbolicTraits(font,size,NULL,traits,kCTFontBoldTrait|kCTFontItalicTrait);
 if (styled != NULL) {CFRelease(font);font=styled;}
 UniChar chars[10]={'0','1','2','3','4','5','6','7','8','9'};CGGlyph glyphs[10];CGSize advances[10];
 double maximum=0;
 if (CTFontGetGlyphsForCharacters(font,chars,glyphs,10)) {
  CTFontGetAdvancesForGlyphs(font,kCTFontOrientationHorizontal,glyphs,advances,10);
  for(int i=0;i<10;i++) if(advances[i].width>maximum) maximum=advances[i].width;
 }
 CFRelease(font);
 return maximum*4.0/3.0;
}
*/
import "C"

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"ww-excel/internal/model"
)

var digitWidthCache sync.Map

func fontDigitWidth(font model.Font) float64 {
	if font.Size <= 0 {
		font.Size = 11
	}
	name := strings.ToLower(strings.TrimSpace(font.Name))
	// Keep the canonical Office default stable across font hinting engines.
	if name == "calibri" && !font.Bold && !font.Italic {
		return 7 * font.Size / 11
	}
	if cached, ok := digitWidthCache.Load(font); ok {
		return cached.(float64)
	}
	family := font.Name
	var filenames []string
	switch name {
	case "微軟正黑體", "microsoft jhenghei", "microsoft jhenghei ui":
		family = "PingFang TC"
		if font.Bold {
			filenames = []string{"MSJHBD.ttf"}
		} else {
			filenames = []string{"MSJH.ttf"}
		}
	case "century gothic":
		if font.Bold {
			filenames = []string{"Century Gothic Bold.ttf"}
		} else {
			filenames = []string{"Century Gothic.ttf"}
		}
	}
	var file *C.char
	// Office's private fonts can supply source metrics without installing or
	// redistributing them. CoreText provides a local fallback on other Macs.
	for _, filename := range filenames {
		path := filepath.Join("/Applications/Microsoft Excel.app/Contents/Resources/DFonts", filename)
		if _, err := os.Stat(path); err == nil {
			file = C.CString(path)
			defer C.free(unsafe.Pointer(file))
			break
		}
	}
	nameC := C.CString(family)
	defer C.free(unsafe.Pointer(nameC))
	bold, italic := 0, 0
	if font.Bold {
		bold = 1
	}
	if font.Italic {
		italic = 1
	}
	width := float64(C.digitAdvance(nameC, file, C.double(font.Size), C.int(bold), C.int(italic)))
	if width <= 0 || width > 1000 {
		width = 7 * font.Size / 11
	}
	digitWidthCache.Store(font, width)
	return width
}
