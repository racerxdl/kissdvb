package h264

import "image"

type VideoFrame struct {
	Frame *image.RGBA
	PTS   float64
}

func MakeVideoFrame(frame *image.RGBA, pts float64) *VideoFrame {
	return &VideoFrame{
		Frame: frame,
		PTS:   pts,
	}
}
