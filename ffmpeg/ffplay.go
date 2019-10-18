package ffmpeg

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
var PFormatContext *avformat.Context
var PCodecContext *avcodec.Context

var YUVFrameMutex sync.Mutex
var PFrameYUV *avutil.Frame

func InitVideo(flpath string) {
	println("open video:" + flpath)

	avformat.AvRegisterAll()
	avcodec.AvcodecRegisterAll()
	PFormatContext = avformat.AvformatAllocContext()
	if avformat.AvformatOpenInput(&PFormatContext, flpath, nil, nil) != 0 {
		fmt.Printf("Unable to open file %s\n", flpath)
		os.Exit(1)
	}

	// Retrieve stream information
	if PFormatContext.AvformatFindStreamInfo(nil) < 0 {
		fmt.Println("Couldn't find stream information")
		os.Exit(1)
	}

	// Dump information about file onto standard error
	PFormatContext.AvDumpFormat(0, flpath, 0)

	// Find the first video stream
nbfor:
	for i := 0; i < int(PFormatContext.NbStreams()); i++ {
		switch PFormatContext.Streams()[i].CodecParameters().AvCodecGetType() {
		case avformat.AVMEDIA_TYPE_VIDEO:
			videoindex = i
			break nbfor
		}
	}
}

func StartVideo() {
	if videoindex < 0 {
		return
	}
	pCodecCtxOrig := PFormatContext.Streams()[videoindex].Codec()
	// Find the decoder for the video stream
	pCodec := avcodec.AvcodecFindDecoder(avcodec.CodecId(pCodecCtxOrig.GetCodecId()))
	if pCodec == nil {
		fmt.Println("Unsupported codec!")
		os.Exit(1)
	}
	// Copy context
	pCodecCtx := pCodec.AvcodecAllocContext3()
	if pCodecCtx.AvcodecCopyContext((*avcodec.Context)(unsafe.Pointer(pCodecCtxOrig))) != 0 {
		fmt.Println("Couldn't copy codec context")
		os.Exit(1)
	}

	// Open codec
	if pCodecCtx.AvcodecOpen2(pCodec, nil) < 0 {
		fmt.Println("Could not open codec")
		os.Exit(1)
	}
	PCodecContext = pCodecCtx

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
		avcodec.AV_PIX_FMT_RGB24,
		avcodec.SWS_BILINEAR,
		nil,
		nil,
		nil,
	)

	// Read frames and save first five frames to disk
	frameNumber := 1
	packet := avcodec.AvPacketAlloc()
	for PFormatContext.AvReadFrame(packet) >= 0 {
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
					return
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
				time.Sleep(time.Microsecond * 40)
			}
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
}
