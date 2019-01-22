package main

import (
	"github.com/racerxdl/go.fifo"
	"github.com/racerxdl/gorrect"
	"os"
	"sync"
	"sync/atomic"
)

var decoderFifo = fifo.NewQueue()

var packetCount = atomic.Value{}
var rsErrors = atomic.Value{}

var currentRotationN = 0
var currentRotation = complex64(1)
var invertedIq = false
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

func DecodePut(samples []complex64) {
	decoderFifo.UnsafeLock()
	rotationLock.Lock()
	for i := 0; i < len(samples); i++ {
		c := samples[i]
		c *= currentRotation
		b0 := float2byte(real(c))
		b1 := float2byte(imag(c))

		if !invertedIq {
			decoderFifo.UnsafeAdd(b0)
			decoderFifo.UnsafeAdd(b1)
		} else {
			decoderFifo.UnsafeAdd(b1)
			decoderFifo.UnsafeAdd(b0)
		}
	}
	rotationLock.Unlock()

	TryDecode()
	decoderFifo.UnsafeUnlock()
}

func UpdateCurrentRotation(rot int, conj bool) {
	rotationLock.Lock()
	baseRotation := rot % 4
	currentRotation = complex64(1)

	if baseRotation != 0 {
		for i := 0; i < 4-baseRotation; i++ {
			currentRotation *= rotation90
		}
	}

	invertedIq = conj
	currentRotationN = rot
	rotationLock.Unlock()
}

var frameBuffer = make([]byte, dvbsFrameBits*scanPackets*2)

var defec = MakeDeFEC()

var deinterleaver = MakeDeinterleaver()

var rs = gorrect.MakeReedSolomon(dvbsFrameSize, mpegtsFrameSize, reedSolomonDistance, reedSolomonPoly)

var mpegtsFrameFifo = fifo.NewQueue()

func TryDecode() {
	if decoderFifo.UnsafeLen() < len(frameBuffer) { // Wait to be able to fill buffer
		return
	}

	for i := 0; i < len(frameBuffer); i++ {
		frameBuffer[i] = decoderFifo.UnsafeNext().(byte)
	}

	defec.PutSoftBits(frameBuffer)
	rot := defec.TryFindSync()

	if defec.IsLocked() && defec.IsFrameReady() {
		Decode(defec.GetLockedFrame())
	}

	if rot != -1 && rot != currentRotationN {
		// TODO: Not working, not sure why
		//conjugated := rot / 4 > 0
		//fmt.Printf("New rotation: %d (was %d)\n", rot, currentRotationN)
		//UpdateCurrentRotation(rot, conjugated)
		//
		//defec.UpdateLockedFrame()
	}
}

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

	// TODO: Fifo to video decoder
	//for i := 0; i < scanPackets; i++ {
	//	mpegtsFrameFifo.Add(dvbFrame[i*mpegtsFrameSize : (i+1)*mpegtsFrameSize])
	//}

	f, err := os.OpenFile("tmpfile", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0770)
	if err != nil {
		panic(err)
	}
	_, err = f.Write(dvbFrame)
	if err != nil {
		panic(err)
	}
	_ = f.Close()
}
