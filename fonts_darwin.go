package main

/*
#cgo LDFLAGS: -framework CoreText -framework CoreFoundation
#include <CoreText/CoreText.h>
#include <stdlib.h>

// fontTables returns the face that CoreText picks for a family name and
// its table tags (an array of uint32), or NULL when the family is not
// installed (CoreText then falls back to another family).
static CTFontRef openFont(const char *family) {
	CFStringRef name = CFStringCreateWithCString(NULL, family, kCFStringEncodingUTF8);
	CTFontRef font = CTFontCreateWithName(name, 12, NULL);
	CFStringRef got = CTFontCopyFamilyName(font);
	CFStringRef localized = CTFontCopyLocalizedName(font, kCTFontFamilyNameKey, NULL);
	Boolean same = CFStringCompare(got, name, kCFCompareCaseInsensitive) == kCFCompareEqualTo ||
		(localized != NULL && CFStringCompare(localized, name, kCFCompareCaseInsensitive) == kCFCompareEqualTo);
	CFRelease(got);
	if (localized != NULL) CFRelease(localized);
	CFRelease(name);
	if (!same) {
		CFRelease(font);
		return NULL;
	}
	return font;
}

static int tableCount(CTFontRef font) {
	CFArrayRef tags = CTFontCopyAvailableTables(font, kCTFontTableOptionNoOptions);
	if (tags == NULL) return 0;
	int n = (int)CFArrayGetCount(tags);
	CFRelease(tags);
	return n;
}

static uint32_t tableTag(CTFontRef font, int i) {
	CFArrayRef tags = CTFontCopyAvailableTables(font, kCTFontTableOptionNoOptions);
	uint32_t tag = (uint32_t)(uintptr_t)CFArrayGetValueAtIndex(tags, i);
	CFRelease(tags);
	return tag;
}

// tableData copies a table into a malloc'ed buffer.
static void *tableData(CTFontRef font, uint32_t tag, long *length) {
	CFDataRef data = CTFontCopyTable(font, tag, kCTFontTableOptionNoOptions);
	if (data == NULL) {
		*length = 0;
		return NULL;
	}
	*length = CFDataGetLength(data);
	void *buf = malloc(*length);
	CFDataGetBytes(data, CFRangeMake(0, *length), buf);
	CFRelease(data);
	return buf;
}

static void closeFont(CTFontRef font) { CFRelease(font); }
*/
import "C"

import (
	"encoding/binary"
	"errors"
	"sort"
	"unsafe"
)

// fontData returns an installed font family as a standalone TrueType
// or OpenType file, built from the tables CoreText reads. This makes
// fonts that WebKit does not offer to pages (macOS's downloadable fonts
// such as 楷體 and 標楷體) usable through FontFace.
func fontData(family string) ([]byte, error) {
	cs := C.CString(family)
	defer C.free(unsafe.Pointer(cs))
	font := C.openFont(cs)
	if font == 0 {
		return nil, errors.New("font not installed")
	}
	defer C.closeFont(font)
	type table struct {
		tag  uint32
		data []byte
	}
	var tables []table
	for i := 0; i < int(C.tableCount(font)); i++ {
		tag := uint32(C.tableTag(font, C.int(i)))
		var n C.long
		buf := C.tableData(font, C.uint32_t(tag), &n)
		if buf == nil {
			continue
		}
		tables = append(tables, table{tag, C.GoBytes(buf, C.int(n))})
		C.free(buf)
	}
	if len(tables) == 0 {
		return nil, errors.New("no font tables")
	}
	sort.Slice(tables, func(i, j int) bool { return tables[i].tag < tables[j].tag })

	// sfnt header and table directory, then 4-byte aligned table data.
	version := uint32(0x00010000)
	for _, t := range tables {
		if t.tag == 0x43464620 { // 'CFF '
			version = 0x4F54544F // 'OTTO'
		}
	}
	n := len(tables)
	entry := 1
	for entry*2 <= n {
		entry *= 2
	}
	searchRange := entry * 16
	log2 := 0
	for 1<<(log2+1) <= entry {
		log2++
	}
	out := make([]byte, 12+16*n)
	binary.BigEndian.PutUint32(out[0:], version)
	binary.BigEndian.PutUint16(out[4:], uint16(n))
	binary.BigEndian.PutUint16(out[6:], uint16(searchRange))
	binary.BigEndian.PutUint16(out[8:], uint16(log2))
	binary.BigEndian.PutUint16(out[10:], uint16(n*16-searchRange))
	for i, t := range tables {
		rec := out[12+16*i:]
		binary.BigEndian.PutUint32(rec[0:], t.tag)
		binary.BigEndian.PutUint32(rec[4:], checksum(t.data))
		binary.BigEndian.PutUint32(rec[8:], uint32(len(out)))
		binary.BigEndian.PutUint32(rec[12:], uint32(len(t.data)))
		out = append(out, t.data...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
	}
	return out, nil
}

func checksum(b []byte) uint32 {
	var sum uint32
	for i := 0; i < len(b); i += 4 {
		var w [4]byte
		copy(w[:], b[i:])
		sum += binary.BigEndian.Uint32(w[:])
	}
	return sum
}
