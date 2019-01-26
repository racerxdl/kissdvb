package h264

import (
	"fmt"
	"github.com/racerxdl/go.fifo"
	"math"
)

const audioPreBufferSize = 1024 * 1024

type H264Parser struct {
	frames      *fifo.Queue
	audioFrames *fifo.Queue
	imageParams *ImageParams
	buffer      []byte
	decoder     *H264Decoder

	audioBuffer []byte
	aacDecoder  *AACDecoder
	audioParams *AudioParams
}

func MakeH264Parser() *H264Parser {
	aacDecoder, err := NewAACDecoder()
	if err != nil {
		fmt.Printf("Error creating audio decoder: %s\n", err)
		return nil
	}

	return &H264Parser{
		frames:      fifo.NewQueue(),
		audioFrames: fifo.NewQueue(),
		buffer:      make([]byte, 0),
		audioBuffer: make([]byte, 0),
		aacDecoder:  aacDecoder,
	}
}

func (p *H264Parser) PutAudioBytes(data []byte) {
	p.audioBuffer = append(p.audioBuffer, data...)
	if p.audioParams == nil && len(p.audioBuffer) > audioPreBufferSize || p.audioParams != nil {
		p.parseAudio()
	}
}

func (p *H264Parser) GetAudioParams() *AudioParams {
	return p.audioParams
}

func (p *H264Parser) PutBytes(data []byte) {
	p.buffer = append(p.buffer, data...)
	p.parse()
}

func (p *H264Parser) NextFramePair() (vf *VideoFrame, af *AudioFrame) {
	if p.frames.Len() == 0 || p.audioFrames.Len() == 0 {
		return
	}
	vf = p.frames.Next().(*VideoFrame)
	af = p.audioFrames.Next().(*AudioFrame)

	return
}

func (p *H264Parser) NextAudioFrame() *AudioFrame {
	if p.audioFrames.Len() > 0 {
		return p.audioFrames.Next().(*AudioFrame)
	}

	return nil
}

func (p *H264Parser) NextFrame() *VideoFrame {
	if p.frames.Len() > 0 {
		return p.frames.Next().(*VideoFrame)
	}

	return nil
}

func (p *H264Parser) parseAudio() {
	if p.aacDecoder == nil {
		return
	}

	p.aacDecoder.SendPacket(p.audioBuffer)
	p.audioBuffer = make([]byte, 0)

	running := true

	for running {
		samples, err := p.aacDecoder.GetFrame()
		if err != nil {
			if err.Error() != "no audio" {
				fmt.Printf("Error decoding audio: %s\n", err)
			}
			running = false
			break
		}

		//fmt.Println("New frame!")
		p.audioFrames.Add(samples)

		if p.audioParams == nil {
			p.audioParams = &AudioParams{
				SamplesPerBuffer: len(samples.Samples),
				SampleRate:       samples.SampleRate,
			}
		}
	}
}

func (p *H264Parser) parse() {
	if p.imageParams == nil {
		// Let's try to get size
		im := GetImageParams(p.buffer)
		if im.OK() {
			// Great!
			fmt.Printf("Got Image Params\n")
			p.imageParams = im
		}
	}

	if p.imageParams != nil {
		if p.decoder == nil {
			// Let's parse frames.
			decoder, err := NewH264Decoder(p.Width(), p.Height(), p.Timebase(), p.StartTime())
			if err != nil {
				fmt.Printf("Error creating decoder: %s\n", err)
				return
			}
			p.decoder = decoder
		}
	}

	if p.decoder != nil {
		p.decoder.SendPacket(p.buffer)
		p.buffer = make([]byte, 0)

		running := true

		for running {
			img, err := p.decoder.GetFrame()
			if err != nil {
				if err.Error() != "no picture" {
					fmt.Printf("Error decoding image: %s\n", err)
				}
				running = false
				break
			}

			//fmt.Println("New frame!")
			p.frames.Add(img)
		}
	}
}

func (p *H264Parser) Width() int {
	if p.imageParams != nil {
		return p.imageParams.Width()
	}

	return -1
}

func (p *H264Parser) Height() int {
	if p.imageParams != nil {
		return p.imageParams.Height()
	}

	return -1
}

func (p *H264Parser) Timebase() float64 {
	if p.imageParams != nil {
		return p.imageParams.Timebase()
	}

	return -1
}

func (p *H264Parser) StartTime() int64 {
	if p.imageParams != nil {
		return p.imageParams.StartTime()
	}

	return -1
}

func (p *H264Parser) FrameRate() float32 {
	if p.imageParams != nil {
		return p.imageParams.FrameRate()
	}

	return float32(math.NaN())
}
