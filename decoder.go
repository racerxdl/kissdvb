package main

import (
	"fmt"
	"github.com/racerxdl/go.fifo"
	"github.com/racerxdl/gorrect"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var packetCount = atomic.Value{}
var rsErrors = atomic.Value{}
var rotationLock = sync.Mutex{}

func float2byte(v float32) byte {

	v = v*127 + 127
	if v > 255 {
		v = 255
	}

	if v < 0 {
		v = 0
	}

	return byte(v)
}

var lastBufferSize int32 = 64 * 1024
var decoderFifo = fifo.NewQueue()

var reusableBuffer = sync.Pool{
	New: func() interface{} {
		size := atomic.LoadInt32(&lastBufferSize)
		return make([]byte, size)
	},
}

func DecodePut(samples []complex64) {
	rotationLock.Lock()

	data := reusableBuffer.Get().([]byte)

	if len(data) < len(samples)*2 {
		fmt.Printf("Not enough bytes in reusableBuffer. Need %d got %d\n", len(samples)*2, len(data))
		atomic.StoreInt32(&lastBufferSize, int32(len(samples)*2))
		data = make([]byte, len(samples)*2)
	}

	for i := 0; i < len(samples); i++ {
		c := samples[i]
		b0 := float2byte(real(c))
		b1 := float2byte(imag(c))

		data[i*2] = b0
		data[i*2+1] = b1
	}

	decoderFifo.Add(data[:len(samples)*2])
	rotationLock.Unlock()
}

func DecodeLoop() {
	for {
		for decoderFifo.Len() > 0 {
			buffer := decoderFifo.Next().([]byte)
			defec.PutSoftBits(buffer)
			size := atomic.LoadInt32(&lastBufferSize)

			if int32(len(buffer)) >= size {
				reusableBuffer.Put(buffer)
			}

			_ = defec.TryFindSync()

			if defec.IsLocked() && defec.IsFrameReady() {
				Decode(defec.GetLockedFrame())
			}
		}
		time.Sleep(time.Microsecond)
		runtime.Gosched()
	}
}

var defec = MakeDeFEC()

var deinterleaver = MakeDeinterleaver()

var rs = gorrect.MakeReedSolomon(dvbsFrameSize, mpegtsFrameSize, reedSolomonDistance, reedSolomonPoly)

func Decode(frame []byte) {
	deinterleaver.PutData(frame)

	if deinterleaver.NumStoredFrames() < scanPackets {
		return
	}

	frames := make([][]byte, scanPackets)

	for i := 0; i < scanPackets; i++ {
		frames[i] = deinterleaver.GetFrame()
		packetCount.Store(packetCount.Load().(int) + 1)
	}

	dvbFrame := make([]byte, mpegtsFrameSize*scanPackets)

	rserrors := 0

	for i := 0; i < scanPackets; i++ {
		decoded, errors := rs.Decode(frames[i])
		copy(dvbFrame[i*mpegtsFrameSize:], decoded)
		rserrors += errors
	}

	rsErrors.Store(rserrors)

	DeRandomize(dvbFrame)

	for i := 0; i < scanPackets; i++ {
		videoPlayer.PutTSFrame(dvbFrame[i*mpegtsFrameSize : (i+1)*mpegtsFrameSize])
	}
}
