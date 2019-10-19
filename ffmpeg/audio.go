package ffmpeg

//#include<memory.h>
import "C"

import (
	"github.com/mgr9525/go-sdl2/sdl"
	"unsafe"
)

//export audioFunc
func audioFunc(userdata unsafe.Pointer, stream *uint8, size int) {
	println("audioFunc", size)

	C.memset(unsafe.Pointer(stream), 0, C.uint(size))
	if AudioBuffer == nil {
		return
	}

	AudioPckMutex.Lock()
	sdl.MixAudio(stream, AudioBuffer, MAX_AUDIO_FRAME_SIZE, sdl.MIX_MAXVOLUME)
	AudioBuffer = nil
	AudioPckMutex.Unlock()
}
