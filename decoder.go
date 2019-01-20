package main

import (
    "github.com/racerxdl/go.fifo"
    "log"
    "os"
)

var decoderFifo = fifo.NewQueue()

func float2byte(v float32) byte {
    v = v * 127 + 127
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
        c := samples[i] // * rotation45 // Rotate 45 degree to align I and Q with bits
        for i := 0; i < 2; i++ {
            c = c * rotation90
        }
        b0 := float2byte(real(c))
        b1 := float2byte(imag(c))

        decoderFifo.UnsafeAdd(b1)
        decoderFifo.UnsafeAdd(b0)
    }

    TryDecode()
    decoderFifo.UnsafeUnlock()
}

var of *os.File

var frameBuffer = make([]byte, dvbsFrameBits * 2)

var defec = MakeDeFEC()

var deinterleaver = MakeDeinterleaver()

func TryDecode() {

    if decoderFifo.UnsafeLen() < len(frameBuffer) { // Wait to be able to fill buffer
        return
    }

    for i := 0; i < len(frameBuffer); i++ {
        frameBuffer[i] = decoderFifo.UnsafeNext().(byte)
    }

    defec.PutSoftBits(frameBuffer)
    _ = defec.TryFindSync()

    if defec.IsLocked() {
        Decode(defec.GetLockedFrame())
    }
}

func Decode(frame []byte) {
    // Decode Data
    if len(frame) != scanPackets * dvbsFrameSize {
        log.Printf("Expected %d got %d in frame size.\n", scanPackets * dvbsFrameSize, len(frame))
        return
    }

    deinterleaver.PutData(frame)

    if deinterleaver.NumStoredFrames() < scanPackets {
        return
    }

    frames := make([][]byte, scanPackets)

    for i := 0; i < scanPackets; i++ {
        frames[i] = deinterleaver.GetFrame()
    }

    // TODO Reed Solomon

    dvbFrame := make([]byte, mpegtsFrameSize * scanPackets)

    for i := 0; i < scanPackets; i++ {
        copy(dvbFrame[i*mpegtsFrameSize:], frames[i][:mpegtsFrameSize])
    }

    DeRandomize(dvbFrame)

    f, err := os.OpenFile("tmpfile", os.O_APPEND | os.O_CREATE | os.O_WRONLY, 0770)
    if err != nil {
        panic(err)
    }
    _, err = f.Write(dvbFrame)
    if err != nil {
        panic(err)
    }
    _ = f.Close()
}
