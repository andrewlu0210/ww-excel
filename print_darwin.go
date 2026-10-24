package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework Quartz
#include <stdlib.h>
#import <Cocoa/Cocoa.h>
#import <Quartz/Quartz.h>

// printPDF shows the system print dialog for a PDF file, attached to
// the key window. The file is removed afterwards.
static void printPDF(const char *cpath) {
	NSString *path = [NSString stringWithUTF8String:cpath];
	dispatch_async(dispatch_get_main_queue(), ^{
		NSURL *url = [NSURL fileURLWithPath:path];
		PDFDocument *doc = [[PDFDocument alloc] initWithURL:url];
		if (doc == nil) {
			return;
		}
		NSPrintInfo *info = [[NSPrintInfo sharedPrintInfo] copy];
		info.topMargin = 0;
		info.bottomMargin = 0;
		info.leftMargin = 0;
		info.rightMargin = 0;
		NSPrintOperation *op = [doc printOperationForPrintInfo:info scalingMode:kPDFPrintPageScaleDownToFit autoRotate:YES];
		op.showsPrintPanel = YES;
		op.showsProgressPanel = YES;
		NSWindow *window = [NSApp keyWindow];
		if (window != nil) {
			[op runOperationModalForWindow:window delegate:nil didRunSelector:nil contextInfo:nil];
		} else {
			[op runOperation];
		}
		[[NSFileManager defaultManager] removeItemAtPath:path error:nil];
	});
}
*/
import "C"

import "unsafe"

// printPDF opens the macOS print dialog for a PDF file.
func printPDF(path string) {
	cs := C.CString(path)
	defer C.free(unsafe.Pointer(cs))
	C.printPDF(cs)
}
