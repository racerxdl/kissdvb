package h264

import (
	/*
	   #cgo LDFLAGS: -lavcodec -lavutil -lavformat -lswscale
	   #cgo CFLAGS: -O2
	   #include "h264.h"
	*/
	"C"
	"errors"
	"fmt"
	"image"
	"unsafe"
)

func init() {
	C.libav_init()
}

type ImageParams struct {
	ip C.image_params_t
}

func (ip *ImageParams) Width() int {
	return int(C.image_params_width(&ip.ip))
}

func (ip *ImageParams) Height() int {
	return int(C.image_params_height(&ip.ip))
}

func (ip *ImageParams) OK() bool {
	return int(C.image_params_ok(&ip.ip)) > 0
}

func (ip *ImageParams) FrameRate() float32 {
	return float32(C.image_params_frameRate(&ip.ip))
}

func (ip *ImageParams) Timebase() float64 {
	return float64(C.image_params_timebase(&ip.ip))
}

func (ip *ImageParams) StartTime() int64 {
	return int64(C.image_params_starttime(&ip.ip))
}

func GetImageParams(data []byte) *ImageParams {
	return &ImageParams{
		ip: C.getImageParams((*C.uint8_t)(unsafe.Pointer(&data[0])), (C.int)(len(data))),
	}
}

type H264Decoder struct {
	m         C.h264dec_t
	tmpBuffer []byte
	width     int
	height    int
}

func NewH264Decoder(width, height int, timebase float64, starttime int64) (m *H264Decoder, err error) {
	fmt.Printf("NewH264Decoder(%d,%d,%f,%d)\n", width, height, timebase, starttime)
	m = &H264Decoder{
		tmpBuffer: make([]byte, width*height*4), // RGBA
		width:     width,
		height:    height,
	}
	r := C.h264dec_new(
		&m.m,
		C.int(width),
		C.int(height),
		C.double(timebase),
		C.long(starttime),
	)
	if int(r) < 0 {
		err = errors.New("open codec failed")
	}
	return
}

func (m *H264Decoder) SendPacket(packet []byte) int {
	r := C.h264dec_sendpacket(
		&m.m,
		(*C.uint8_t)(unsafe.Pointer(&packet[0])),
		(C.int)(len(packet)))

	return int(r)
}

func (m *H264Decoder) GetFrame() (vf *VideoFrame, err error) {
	//runtime.LockOSThread()
	C.h264dec_recvpacket(&m.m, (*C.uint8_t)(unsafe.Pointer(&m.tmpBuffer[0])), (C.int)(len(m.tmpBuffer)))
	//runtime.UnlockOSThread()
	if m.m.got == 0 {
		err = errors.New("no picture")
		return
	}

	m.m.got = 0

	f := image.NewRGBA(image.Rect(0, 0, m.width, m.height))

	copy(f.Pix, m.tmpBuffer)

	vf = MakeVideoFrame(f, float64(m.m.pts))

	return
}
