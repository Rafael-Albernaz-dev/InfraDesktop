package storage

/*
#cgo pkg-config: libwebp
#include <webp/decode.h>
#include <stdlib.h>
static int valid_brand_webp(const unsigned char *data, size_t size) {
 int w=0,h=0;
 if (!WebPGetInfo(data,size,&w,&h) || w<1 || h<1 || w>4096 || h>4096) return 0;
 size_t capacity=(size_t)w*(size_t)h*4;
 unsigned char *pixels=(unsigned char *)malloc(capacity);
 if (!pixels) return 0;
 int ok=WebPDecodeRGBAInto(data,size,pixels,capacity,w*4)!=NULL;
 free(pixels);
 return ok;
}
*/
import "C"
import "unsafe"

// The explicitly authorized, installed libwebp fully decodes bounded pixels.
// No codec pointers or technology errors leave this adapter.
func validWebP(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	return C.valid_brand_webp((*C.uchar)(unsafe.Pointer(&data[0])), C.size_t(len(data))) == 1
}
