package main

import (
	"github.com/racerxdl/go.fifo"
	"github.com/racerxdl/gorrect"
	"github.com/racerxdl/gorrect/Codes"
	"os"
	"sync/atomic"
)

var decoderFifo = fifo.NewQueue()

var packetCount = atomic.Value{}
var rsErrors = atomic.Value{}

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
	for i := 0; i < len(samples); i++ {
		c := samples[i]
		for i := 0; i < 2; i++ {
			c = c * rotation90
		}
		b0 := float2byte(real(c))
		b1 := float2byte(imag(c))

		decoderFifo.UnsafeAdd(b0)
		decoderFifo.UnsafeAdd(b1)
	}

	TryDecode()
	decoderFifo.UnsafeUnlock()
}

var frameBuffer = make([]byte, dvbsFrameBits*scanPackets*2)

var defec = MakeDeFEC()

var deinterleaver = MakeDeinterleaver()

var rs = gorrect.MakeReedSolomon(204, 188, 8, Codes.ReedSolomonPrimitivePolynomial8_4_3_2_0)

func TryDecode() {
	if decoderFifo.UnsafeLen() < len(frameBuffer) { // Wait to be able to fill buffer
		return
	}

	for i := 0; i < len(frameBuffer); i++ {
		frameBuffer[i] = decoderFifo.UnsafeNext().(byte)
	}

	defec.PutSoftBits(frameBuffer)
	_ = defec.TryFindSync()

	if defec.IsLocked() && defec.IsFrameReady() {
		Decode(defec.GetLockedFrame())
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
