package main

import (
	"context"
	"fmt"
	"github.com/asticode/go-astits"
	"github.com/gordonklaus/portaudio"
	"github.com/racerxdl/kissdvb/h264"
	"image"
	"sync"
	"time"
)

const (
	StreamTypeVideo = 0x1B
	StreamTypeAudio = 0x0F
)

const MinProbeLength = 128 * 1024 // 1 MB

func init() {
	err := portaudio.Initialize()
	if err != nil {
		panic(err)
		return
	}
}

type VideoPlayer struct {
	audioStream    *portaudio.Stream
	videoStreamPID int
	audioStreamPID int

	frameParser   *h264.H264Parser
	videoFrame    *image.RGBA
	newFrameReady bool
	currFrameTime float64

	frameSync  sync.Mutex
	cancel     context.CancelFunc
	fifoReader *FifoReader

	width  int
	height int
}

func MakeVideoPlayer() *VideoPlayer {
	return &VideoPlayer{
		videoStreamPID: -1,
		audioStreamPID: -1,
		frameParser:    h264.MakeH264Parser(),
		videoFrame:     image.NewRGBA(image.Rect(0, 0, 640, 480)),
		currFrameTime:  0,
		newFrameReady:  true,
		frameSync:      sync.Mutex{},
		cancel:         nil,
		fifoReader:     MakeFifoReader(),
		width:          640,
		height:         480,
	}
}

func (vp *VideoPlayer) Start() {
	go vp.decodeRoutine()
}

func (vp *VideoPlayer) Stop() {
	if vp.cancel != nil {
		vp.cancel()
	}
}

func (vp *VideoPlayer) Width() int {
	vp.frameSync.Lock()
	defer vp.frameSync.Unlock()

	return vp.width
}

func (vp *VideoPlayer) Height() int {
	vp.frameSync.Lock()
	defer vp.frameSync.Unlock()

	return vp.height
}

func (vp *VideoPlayer) PutTSFrame(ts []byte) {
	vp.fifoReader.PutData(ts)
}

func (vp *VideoPlayer) IsFrameReady() bool {
	vp.frameSync.Lock()
	defer vp.frameSync.Unlock()

	return vp.newFrameReady
}

func (vp *VideoPlayer) processAudio(out []float32) {
	af := vp.frameParser.NextAudioFrame()
	if af != nil {
		copy(out, af.Samples)
	}
}

func (vp *VideoPlayer) startAudio(sampleRate float64, numSamples int) error {
	h, err := portaudio.DefaultHostApi()

	if err != nil {
		return err
	}

	fmt.Printf("Starting audio with %f Hz SampleRate and %d buffer length\n", sampleRate, numSamples)
	p := portaudio.HighLatencyParameters(nil, h.DefaultOutputDevice)
	p.Input.Channels = 0
	p.Output.Channels = 1
	p.SampleRate = sampleRate
	p.FramesPerBuffer = numSamples

	// Add few empty buffers to keep up on start

	vp.audioStream, err = portaudio.OpenStream(p, vp.processAudio)
	return vp.audioStream.Start()
}

func (vp *VideoPlayer) stopAudio() {
	if vp.audioStream != nil {
		_ = vp.audioStream.Stop()
		vp.audioStream = nil
	}
}

func (vp *VideoPlayer) putAudioData(data []byte) {
	vp.frameParser.PutAudioBytes(data)
	if vp.audioStream == nil {
		audioParams := vp.frameParser.GetAudioParams()
		if audioParams != nil {
			err := vp.startAudio(audioParams.SampleRate, audioParams.SamplesPerBuffer)
			if err != nil {
				fmt.Printf("Error starting audio: %s\n", err)
				vp.audioStream = nil
			}
		}
	}
}

func (vp *VideoPlayer) putVideoData(data []byte) {
	vp.frameParser.PutBytes(data)
	vf := vp.frameParser.NextFrame()

	if vf != nil {
		sleepTime := vf.PTS - vp.currFrameTime

		if sleepTime > 0 {
			time.Sleep(time.Duration(sleepTime * float64(time.Second)))
		}

		vp.currFrameTime = vf.PTS

		vp.frameSync.Lock()
		defer vp.frameSync.Unlock()
		dstBounds := vf.Frame.Bounds()

		if dstBounds.Dy() == 0 || dstBounds.Dx() == 0 {
			// Invalid Frame
			return
		}

		vp.width = dstBounds.Dx()
		vp.height = dstBounds.Dy()

		vp.videoFrame = vf.Frame
		vp.newFrameReady = true
	}
}

func (vp *VideoPlayer) GetFrame() *image.RGBA {
	vp.frameSync.Lock()
	defer vp.frameSync.Unlock()

	if vp.newFrameReady {
		vp.newFrameReady = false
		return vp.videoFrame
	}

	return nil
}

func (vp *VideoPlayer) decodeRoutine() {
	ctx, cancel := context.WithCancel(context.Background())

	vp.cancel = cancel

	dmx := astits.New(ctx, vp.fifoReader)

	for {
		if vp.fifoReader.Len() < MinProbeLength {
			time.Sleep(time.Microsecond)
			continue
		}
		// Get the next data
		d, err := dmx.NextData()

		if err != nil {
			fmt.Println(err)
			break
		}

		// Data is a PMT data
		if d.PMT != nil && vp.videoStreamPID == -1 {
			fmt.Printf("Program %d: \n", d.PMT.ProgramNumber)
			//Loop through elementary streams
			for _, es := range d.PMT.ElementaryStreams {
				fmt.Printf("    Stream detected: %d\n", es.ElementaryPID)
				fmt.Printf("    Stream Type: 0x%02x\n", es.StreamType)

				if es.StreamType == StreamTypeVideo {
					vp.videoStreamPID = int(es.ElementaryPID)
				} else if es.StreamType == StreamTypeAudio {
					vp.audioStreamPID = int(es.ElementaryPID)
				}

				for _, esd := range es.ElementaryStreamDescriptors {
					fmt.Printf("        %v\n", esd)
				}
			}
		}

		if d.PES != nil && vp.videoStreamPID != -1 {
			if d.PID == uint16(vp.videoStreamPID) {
				vp.putVideoData(d.PES.Data)
			} else if d.PID == uint16(vp.audioStreamPID) {
				vp.putAudioData(d.PES.Data)
			}
		}
	}
}
