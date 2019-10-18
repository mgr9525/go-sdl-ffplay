package main

import "C"
import (
	"fmt"
	"github.com/mgr9525/go-sdl2/sdl"
	"github.com/mgr9525/goav/avutil"
	"go-sdl-ffplay/app"
	"go-sdl-ffplay/ffmpeg"
	"log"
	"os"
	"path/filepath"
)

func main() {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("AppPath:", dir)
	app.Path = dir

	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
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

	render, err := sdl.CreateRenderer(window, 0, 0)
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
						tetr, err := render.CreateTexture(sdl.PIXELFORMAT_IYUV, sdl.TEXTUREACCESS_STREAMING, int32(ffmpeg.PCodecContext.CodedWidth()), int32(ffmpeg.PCodecContext.CodedHeight()))
						if err != nil {
							panic(err)
						}
						app.FTexture = tetr
					}

					if ffmpeg.PFrameYUV != nil {
						img, err := avutil.GetPicture(ffmpeg.PFrameYUV)
						if err == nil {
							app.FTexture.Update(nil, img.Y, img.Rect.Max.X)
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
