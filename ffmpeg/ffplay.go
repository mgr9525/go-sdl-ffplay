package ffmpeg

/*
#include<stdio.h>
#include<SDL2/SDL_audio.h>
void audioCallback(void *userdata, Uint8 * stream,int len){
	printf("audioCallback\n");
	audioFunc(userdata,stream,len);
}
*/
import "C"
import (
	"fmt"
	"github.com/mgr9525/go-sdl2/sdl"
	"github.com/mgr9525/goav/avcodec"
	"github.com/mgr9525/goav/avformat"
	"github.com/mgr9525/goav/avutil"
	"github.com/mgr9525/goav/swscale"
	"go-sdl-ffplay/app"
	"os"
	"sync"
	"time"
	"unsafe"
)

var videoindex = -1
var audioindex = -1
var PFormatContext *avformat.Context
var PCodecContext *avcodec.Context
var PCodecCtxAud *avcodec.Context

var YUVFrameMutex sync.Mutex
var PFrameYUV *avutil.Frame

var PAudioPacket *avcodec.Packet
var AudioPckMutex sync.Mutex

func InitVideo(flpath string) {
	println("open video:" + flpath)

	pFormatContext := avformat.AvformatAllocContext()
	if avformat.AvformatOpenInput(&pFormatContext, flpath, nil, nil) != 0 {
		fmt.Printf("Unable to open file %s\n", flpath)
		os.Exit(1)
	}
	PFormatContext = pFormatContext

	// Retrieve stream information
	if PFormatContext.AvformatFindStreamInfo(nil) < 0 {
		fmt.Println("Couldn't find stream information")
		os.Exit(1)
	}

	// Dump information about file onto standard error
	PFormatContext.AvDumpFormat(0, flpath, 0)

	// Find the first video stream
	for i := 0; i < int(PFormatContext.NbStreams()); i++ {
		switch PFormatContext.Streams()[i].CodecParameters().AvCodecGetType() {
		case avformat.AVMEDIA_TYPE_VIDEO:
			videoindex = i
			break
		case avformat.AVMEDIA_TYPE_AUDIO:
			audioindex = i
			break
		}
	}

	println("videoindex:", videoindex)
}

func StartVideo() {
	if videoindex < 0 {
		return
	}
	rate := PFormatContext.Streams()[videoindex].AvgFrameRate()
	secs := float64(rate.Den()) / float64(rate.Num())
	pCodecCtxOrig := PFormatContext.Streams()[videoindex].Codec()
	pCodecCtxAud := PFormatContext.Streams()[audioindex].Codec()
	// Find the decoder for the video stream
	pCodec := avcodec.AvcodecFindDecoder(avcodec.CodecId(pCodecCtxOrig.GetCodecId()))
	if pCodec == nil {
		fmt.Println("Unsupported codec!")
		os.Exit(1)
	}
	pCodecAud := avcodec.AvcodecFindDecoder(avcodec.CodecId(pCodecCtxAud.GetCodecId()))
	if pCodecAud == nil {
		fmt.Println("Unsupported codec audio!")
		os.Exit(1)
	}
	// Copy context
	pCodecCtx := pCodec.AvcodecAllocContext3()
	if pCodecCtx.AvcodecCopyContext((*avcodec.Context)(unsafe.Pointer(pCodecCtxOrig))) != 0 {
		fmt.Println("Couldn't copy codec context")
		os.Exit(1)
	}
	// Copy context
	pCodecCtxAuds := pCodecAud.AvcodecAllocContext3()
	if pCodecCtxAuds.AvcodecCopyContext((*avcodec.Context)(unsafe.Pointer(pCodecCtxAud))) != 0 {
		fmt.Println("Couldn't copy codec context")
		os.Exit(1)
	}

	// Open codec
	if pCodecCtx.AvcodecOpen2(pCodec, nil) < 0 {
		fmt.Println("Could not open codec")
		os.Exit(1)
	}
	PCodecContext = pCodecCtx

	// Open codec
	if pCodecCtxAuds.AvcodecOpen2(pCodecAud, nil) < 0 {
		fmt.Println("Could not open codec audio")
		os.Exit(1)
	}
	PCodecCtxAud = pCodecCtxAuds

	// Allocate video frame
	pFrame := avutil.AvFrameAlloc()

	// Allocate an AVFrame structure
	pFrameYUV := avutil.AvFrameAlloc()
	if pFrameYUV == nil {
		fmt.Println("Unable to allocate YUV Frame")
		os.Exit(1)
	}

	numBytes := uintptr(avcodec.AvpictureGetSize(avcodec.AV_PIX_FMT_YUV, pCodecCtx.Width(),
		pCodecCtx.Height()))
	buffer := avutil.AvMalloc(numBytes)

	// Assign appropriate parts of buffer to image planes in pFrameRGB
	// Note that pFrameRGB is an AVFrame, but AVFrame is a superset
	// of AVPicture
	avp := (*avcodec.Picture)(unsafe.Pointer(pFrameYUV))
	avp.AvpictureFill((*uint8)(buffer), avcodec.AV_PIX_FMT_YUV, pCodecCtx.Width(), pCodecCtx.Height())

	// initialize SWS context for software scaling
	swsCtx := swscale.SwsGetcontext(
		pCodecCtx.Width(),
		pCodecCtx.Height(),
		(swscale.PixelFormat)(pCodecCtx.PixFmt()),
		pCodecCtx.Width(),
		pCodecCtx.Height(),
		avcodec.AV_PIX_FMT_YUV,
		avcodec.SWS_BILINEAR,
		nil,
		nil,
		nil,
	)

	wspec := new(sdl.AudioSpec)
	wspec.Freq = int32(pCodecCtxAuds.SampleRate())
	wspec.Format = sdl.AUDIO_S16SYS
	wspec.Channels = uint8(pCodecCtxAuds.Channels())
	wspec.Silence = 0
	wspec.Samples = 1024
	wspec.UserData = unsafe.Pointer(pCodecCtxAuds)
	wspec.Callback = sdl.AudioCallback(unsafe.Pointer(C.audioCallback))

	err := sdl.OpenAudio(wspec, nil)
	if err != nil {
		fmt.Println("SDL_OpenAudio err:" + err.Error())
		os.Exit(1)
	}

	sdl.PauseAudio(false)

	// Read frames and save first five frames to disk
	frameNumber := 1
	packet := avcodec.AvPacketAlloc()

	for {
		if PFormatContext.AvReadFrame(packet) < 0 {
			break
		}
		// Is this a packet from the video stream?
		if packet.StreamIndex() == videoindex {
			// Decode video frame
			response := pCodecCtx.AvcodecSendPacket(packet)
			if response < 0 {
				fmt.Printf("Error while sending a packet to the decoder: %s\n", avutil.ErrorFromCode(response))
			}
			for response >= 0 {
				response = pCodecCtx.AvcodecReceiveFrame((*avcodec.Frame)(unsafe.Pointer(pFrame)))
				if response == avutil.AvErrorEAGAIN || response == avutil.AvErrorEOF {
					break
				} else if response < 0 {
					fmt.Printf("Error while receiving a frame from the decoder: %s\n", avutil.ErrorFromCode(response))
					break
				}

				YUVFrameMutex.Lock()
				// Convert the image from its native format to RGB
				swscale.SwsScale2(swsCtx, avutil.Data(pFrame),
					avutil.Linesize(pFrame), 0, pCodecCtx.Height(),
					avutil.Data(pFrameYUV), avutil.Linesize(pFrameYUV))
				PFrameYUV = pFrameYUV
				YUVFrameMutex.Unlock()
				sdl.PushEvent(&sdl.UserEvent{Type: app.FEvent, Timestamp: uint32(time.Now().Unix())})

				// Save the frame to disk
				fmt.Printf("Writing frame %d\n", frameNumber)
				//(pFrameRGB, pCodecCtx.Width(), pCodecCtx.Height(), frameNumber)

				frameNumber++
			}

			time.Sleep(time.Millisecond * time.Duration(1000*secs))
		} else if packet.StreamIndex() == audioindex {
			response := pCodecCtxAuds.AvcodecSendPacket(packet)
			if response < 0 {
				fmt.Printf("Error while sending a packet to the decoder: %s\n", avutil.ErrorFromCode(response))
			}
			for response >= 0 {
				response = pCodecCtxAuds.AvcodecReceiveFrame((*avcodec.Frame)(unsafe.Pointer(pFrame)))
				if response == avutil.AvErrorEAGAIN || response == avutil.AvErrorEOF {
					break
				} else if response < 0 {
					fmt.Printf("Error while receiving a frame from the decoder: %s\n", avutil.ErrorFromCode(response))
					break
				}

			}
			AudioPckMutex.Lock()
			PAudioPacket = packet
			AudioPckMutex.Unlock()
		}

		// Free the packet that was allocated by av_read_frame
		packet.AvFreePacket()
	}

	// Free the RGB image
	avutil.AvFree(buffer)
	avutil.AvFrameFree(pFrameYUV)

	// Free the YUV frame
	avutil.AvFrameFree(pFrame)

	// Close the codecs
	pCodecCtx.AvcodecClose()
	(*avcodec.Context)(unsafe.Pointer(pCodecCtxOrig)).AvcodecClose()

	// Close the video file
	PFormatContext.AvformatCloseInput()

	println("read video end!!!!!!!!!!!!!!!!!!!!!!!!!!!11")
}
