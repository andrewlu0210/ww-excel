package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#import <Cocoa/Cocoa.h>

// setAppearance forces light (1) or dark (2) for the whole app, or
// follows the system (0). WKWebView's prefers-color-scheme follows it.
static void setAppearance(int mode) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSAppearance *a = nil;
		if (mode == 1) a = [NSAppearance appearanceNamed:NSAppearanceNameAqua];
		if (mode == 2) a = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
		[NSApp setAppearance:a];
	});
}

// preferredLanguages returns the system language list, one per line.
static char *preferredLanguages(void) {
	NSString *s = [[NSLocale preferredLanguages] componentsJoinedByString:@"\n"];
	return strdup([s UTF8String]);
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

func setAppearance(mode string) {
	switch mode {
	case "light":
		C.setAppearance(1)
	case "dark":
		C.setAppearance(2)
	default:
		C.setAppearance(0)
	}
}

func preferredLanguages() []string {
	p := C.preferredLanguages()
	defer C.free(unsafe.Pointer(p))
	return strings.Split(C.GoString(p), "\n")
}
