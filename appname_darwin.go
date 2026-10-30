package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
#include <stdlib.h>

// AppKit derives the application menu name from the process on bare-binary launches.
// Called on the main thread when the localized menus are rebuilt.
static void wwSetApplicationName(const char *name) {
    NSString *title = [NSString stringWithUTF8String:name];
    [NSProcessInfo processInfo].processName = title;
    // Update only the mutable in-memory bundle metadata, including bare binaries.
    NSDictionary *info = [NSBundle mainBundle].infoDictionary;
    if ([info isKindOfClass:[NSMutableDictionary class]]) {
        [(NSMutableDictionary *)info setObject:title forKey:@"CFBundleName"];
        [(NSMutableDictionary *)info setObject:title forKey:@"CFBundleDisplayName"];
    }
    NSMenuItem *item = NSApp.mainMenu.itemArray.firstObject;
    if (item) {
        item.title = title;
        item.submenu.title = title;
    }
}
*/
import "C"
import "unsafe"

func setApplicationDisplayName(name string) {
	value := C.CString(name)
	defer C.free(unsafe.Pointer(value))
	C.wwSetApplicationName(value)
}
