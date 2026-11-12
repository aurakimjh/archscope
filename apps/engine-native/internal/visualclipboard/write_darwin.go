//go:build darwin && cgo

package visualclipboard

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
#include <stdlib.h>
static int writeVisualClipboard(const void *png, size_t pngLen, const char *html, const char *text) {
 @autoreleasepool {
  NSPasteboardItem *item = [[NSPasteboardItem alloc] init];
  BOOL valid = YES;
  if (pngLen > 0) {
   NSData *data = [NSData dataWithBytes:png length:pngLen];
   NSImage *image = [[NSImage alloc] initWithData:data];
   NSData *tiff = [image TIFFRepresentation];
   valid = image != nil && tiff != nil && [item setData:data forType:NSPasteboardTypePNG] && [item setData:tiff forType:NSPasteboardTypeTIFF];
   [image release];
  } else {
   NSString *h = [NSString stringWithUTF8String:html];
   NSString *t = [NSString stringWithUTF8String:text];
   valid = h != nil && t != nil && [item setString:h forType:NSPasteboardTypeHTML] && [item setString:t forType:NSPasteboardTypeString];
  }
  BOOL wrote = NO;
  if (valid) { NSPasteboard *board = [NSPasteboard generalPasteboard]; [board clearContents]; wrote = [board writeObjects:@[item]]; }
  [item release];
  return wrote ? 1 : 0;
 }
}
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"
)

func Supported() bool { return true }

// Write must run on the desktop UI thread; the caller dispatches through Wails.
func Write(ctx context.Context, owner uintptr, p Payload) error {
	if e := p.Validate(); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	var data unsafe.Pointer
	if len(p.PNG) > 0 {
		data = C.CBytes(p.PNG)
		defer C.free(data)
	}
	h, t := C.CString(p.HTML), C.CString(p.Text)
	defer C.free(unsafe.Pointer(h))
	defer C.free(unsafe.Pointer(t))
	if C.writeVisualClipboard(data, C.size_t(len(p.PNG)), h, t) != 1 {
		return fmt.Errorf("CLIPBOARD_WRITE_FAILED: save PNG/TSV or retry")
	}
	return nil
}
