package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
#include <stdlib.h>

static NSPanel *wwAboutPanel;
static NSImageView *wwAboutIcon;
static NSTextField *wwAboutVersion;
static NSTextField *wwAboutDescription;
static NSTextField *wwAboutCredit;

static NSTextField *wwAboutLabel(CGFloat size, NSFontWeight weight) {
    NSTextField *label = [NSTextField wrappingLabelWithString:@""];
    label.font = [NSFont systemFontOfSize:size weight:weight];
    label.alignment = NSTextAlignmentCenter;
    label.selectable = YES;
    return label;
}

// Uses explicit data rather than NSBundle metadata, including command-line launches.
// Called on the main thread. Keep one nonmodal panel and reuse it on every opening.
static void wwShowAbout(const void *icon, int length, const char *title,
                        const char *version, const char *description, const char *credit) {
    if (!wwAboutPanel) {
        wwAboutPanel = [[NSPanel alloc] initWithContentRect:NSMakeRect(0, 0, 420, 390)
            styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable
            backing:NSBackingStoreBuffered defer:NO];
        wwAboutPanel.releasedWhenClosed = NO;
        wwAboutPanel.hidesOnDeactivate = NO;
        wwAboutPanel.tabbingMode = NSWindowTabbingModeDisallowed;

        NSStackView *stack = [[NSStackView alloc] init];
        stack.orientation = NSUserInterfaceLayoutOrientationVertical;
        stack.alignment = NSLayoutAttributeCenterX;
        stack.spacing = 12;
        stack.translatesAutoresizingMaskIntoConstraints = NO;
        [wwAboutPanel.contentView addSubview:stack];

        wwAboutIcon = [[NSImageView alloc] init];
        wwAboutIcon.imageScaling = NSImageScaleProportionallyUpOrDown;
        wwAboutIcon.accessibilityLabel = @"WW Excel";
        [stack addArrangedSubview:wwAboutIcon];
        [wwAboutIcon.widthAnchor constraintEqualToConstant:104].active = YES;
        [wwAboutIcon.heightAnchor constraintEqualToConstant:104].active = YES;

        NSTextField *name = wwAboutLabel(24, NSFontWeightSemibold);
        name.stringValue = @"WW Excel";
        [stack addArrangedSubview:name];

        wwAboutVersion = wwAboutLabel(12, NSFontWeightRegular);
        wwAboutVersion.textColor = NSColor.secondaryLabelColor;
        [stack addArrangedSubview:wwAboutVersion];

        wwAboutDescription = wwAboutLabel(13, NSFontWeightRegular);
        wwAboutDescription.textColor = NSColor.secondaryLabelColor;
        [stack addArrangedSubview:wwAboutDescription];
        [wwAboutDescription.widthAnchor constraintEqualToConstant:340].active = YES;

        NSBox *separator = [[NSBox alloc] init];
        separator.boxType = NSBoxSeparator;
        [stack addArrangedSubview:separator];
        [separator.widthAnchor constraintEqualToConstant:340].active = YES;

        wwAboutCredit = wwAboutLabel(12, NSFontWeightMedium);
        [stack addArrangedSubview:wwAboutCredit];

        [NSLayoutConstraint activateConstraints:@[
            [stack.centerXAnchor constraintEqualToAnchor:wwAboutPanel.contentView.centerXAnchor],
            [stack.centerYAnchor constraintEqualToAnchor:wwAboutPanel.contentView.centerYAnchor],
            [stack.widthAnchor constraintEqualToConstant:340]
        ]];
        [wwAboutPanel center];
    }
    wwAboutPanel.title = [NSString stringWithUTF8String:title];
    wwAboutIcon.image = [[[NSImage alloc] initWithData:[NSData dataWithBytes:icon length:length]] autorelease];
    wwAboutVersion.stringValue = [NSString stringWithUTF8String:version];
    wwAboutDescription.stringValue = [NSString stringWithUTF8String:description];
    wwAboutCredit.stringValue = [NSString stringWithUTF8String:credit];
    [wwAboutPanel makeKeyAndOrderFront:nil];
    [NSApp activateIgnoringOtherApps:YES];
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

const appDescription = "macOS 試算表工具，支援 Excel 檔案編輯、公式計算與圖表。"
const appCredit = "由台北科研製作"

func showAbout() {
	title := C.CString(T("關於 WW Excel"))
	versionText := C.CString(fmt.Sprintf(T("版本 %s"), version))
	description := C.CString(T(appDescription))
	credit := C.CString(T(appCredit))
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(versionText))
	defer C.free(unsafe.Pointer(description))
	defer C.free(unsafe.Pointer(credit))
	onMain(func() {
		C.wwShowAbout(unsafe.Pointer(&appIcon[0]), C.int(len(appIcon)), title, versionText, description, credit)
	})
}
