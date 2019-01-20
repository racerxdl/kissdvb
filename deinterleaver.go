package main

import "github.com/racerxdl/go.fifo"

type Deinterleaver struct {
    I int
    J int
    buffer []byte
    frameFifo *fifo.Queue
}


func MakeDeinterleaver() *Deinterleaver {
    I := 12
    J := 17
    di := &Deinterleaver{
        I: I,
        J: J,
        buffer: make([]byte, 0),
        frameFifo: fifo.NewQueue(),
    }


    return di
}

func (di *Deinterleaver) PutData(d []byte) {
    di.buffer = append(di.buffer, d...)
    di.process()
}

func (di *Deinterleaver) process() {
    l := di.J * (di.I-1) * di.I + dvbsFrameSize
    for len(di.buffer) >= l {
        out := make([]byte, dvbsFrameSize)

        delay := di.J * (di.I - 1)

        for i := 0; i < dvbsFrameSize; i++ {
            pos :=  di.J * (di.I-1) * di.I // Base position
            pos += i // offset
            pos -= delay * di.I // Line delay

            out[i] = di.buffer[pos]

            delay = (delay - di.J + di.J * di.I) % ( di.J * di.I)
        }

        di.frameFifo.Add(out)
        di.buffer = di.buffer[dvbsFrameSize:]
    }
}

func (di *Deinterleaver) NumStoredFrames() int {
    return di.frameFifo.Len()
}

func (di *Deinterleaver) GetFrame() []byte {
    if di.frameFifo.Len() > 0 {
        return di.frameFifo.Next().([]byte)
    }
    return nil
}