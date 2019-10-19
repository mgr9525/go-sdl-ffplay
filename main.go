package main

import "C"
import (
	"fmt"
	"github.com/mgr9525/go-sdl-ffplay/app"
	"github.com/mgr9525/go-sdl-ffplay/ffmpeg"
	"github.com/mgr9525/go-sdl2/sdl"
	"github.com/mgr9525/goav/avcodec"
	"github.com/mgr9525/goav/avformat"
	"github.com/mgr9525/goav/avutil"
	"log"
	"os"
	"path/filepath"
	"unsafe"
)

func main() {
	avformat.AvRegisterAll()
	avcodec.AvcodecRegisterAll()
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("AppPath:", dir)
	app.Path = dir

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_AUDIO | sdl.INIT_TIMER); err != nil {
		panic(err)
	}
	defer sdl.Quit()

	app.FEvent = sdl.RegisterEvents(1)

	window, err := sdl.CreateWindow("test", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		800, 600, sdl.WINDOW_SHOWN)
	if err != nil {
		panic(err)
	}
	defer window.Destroy()
	surface, err := window.GetSurface()
	if err != nil {
		panic(err)
	}

	render, err := sdl.CreateRenderer(window, 1, 0)
	if err != nil {
		panic(err)
	}

	ffmpeg.InitVideo(app.Path + "\\123.mp4")

	surface.FillRect(nil, 0)
	rect := sdl.Rect{0, 0, 200, 200}
	surface.FillRect(&rect, 0xffff0000)
	window.UpdateSurface()

	go ffmpeg.StartVideo()
	running := true
	for running {
		e := sdl.PollEvent()
		if e == nil {

		} else {
			switch e.(type) {
			case *sdl.UserEvent:
				if e.GetType() == app.FEvent {
					if app.FTexture == nil && ffmpeg.PCodecContext != nil {
						println("CreateTexture", int32(ffmpeg.PCodecContext.Width()), int32(ffmpeg.PCodecContext.Height()))
						tetr, err := render.CreateTexture(sdl.PIXELFORMAT_IYUV, sdl.TEXTUREACCESS_STREAMING, int32(ffmpeg.PCodecContext.Width()), int32(ffmpeg.PCodecContext.Height()))
						if err != nil {
							panic(err)
						}
						app.FTexture = tetr
					}

					if ffmpeg.PFrameYUV != nil {
						ffmpeg.YUVFrameMutex.Lock()
						w, h, sz, data := avutil.AvFrameGetInfo(ffmpeg.PFrameYUV)
						ffmpeg.PFrameYUV = nil
						ffmpeg.YUVFrameMutex.Unlock()
						fmt.Printf("AvFrameGetInfo w:%d,h:%d\n", w, h)
						//bts:=C.GoBytes(unsafe.Pointer(&data[0]), C.int(int(sz[0])+int(sz[1])+int(sz[2])))
						//println("update len:",w,h,len(bts),len(sz),int(sz[0]),int(sz[1]),int(sz[2]))
						//img, err := avutil.GetPicture(ffmpeg.PFrameYUV)
						if err == nil {
							//app.FTexture.Update(nil, bts, int(sz[0]))
							app.FTexture.Updates(nil, unsafe.Pointer(data[0]), int(sz[0]))
							render.Clear()
							render.Copy(app.FTexture, nil, nil)
							render.Present()
						}
					}
				}
			case *sdl.QuitEvent:
				running = false
				println("Quit")
				break
			}
		}
	}
}
