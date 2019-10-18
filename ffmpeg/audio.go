package ffmpeg

import (
	"C"
	"unsafe"
)

//export audioFunc
func audioFunc(userdata unsafe.Pointer, stream *uint8, size int) {
	println("audioFunc", size)
}
