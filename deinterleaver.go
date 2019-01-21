package main

import (
	"github.com/racerxdl/go.fifo"
	"sync"
)

// https://www.etsi.org/deliver/etsi_en/300400_300499/300421/01.01.02_60/en_300421v010102p.pdf page 10

type Deinterleaver struct {
	sync.Mutex

	I         int
	M         int
	frameFifo *fifo.Queue

	diFifo    []*fifo.Queue
	comutator int
	gotSync   bool
	outBuffer []byte
	outN      int
}

func MakeDeinterleaver() *Deinterleaver {
	I := 12
	M := 17

	diFifo := make([]*fifo.Queue, I)

	for i := 0; i < I; i++ {
		diFifo[i] = fifo.NewQueue()
		bi := I - i - 1 // Branch Index, reversed for Interleaver.
		// Fill with pre-state
		for z := 0; z < M*bi; z++ { // M * bi Depth, pre-fill
			diFifo[i].Add(byte(0x00))
		}
	}

	di := &Deinterleaver{
		I:         I,
		M:         M,
		frameFifo: fifo.NewQueue(),
		diFifo:    diFifo,
		comutator: 0,
		outBuffer: make([]byte, dvbsFrameSize),
		outN:      0,
	}

	return di
}

func (di *Deinterleaver) PutData(d []byte) {
	di.Lock()

	for i := 0; i < len(d); i++ {
		di.diFifo[di.comutator].Add(d[i])
		b := di.diFifo[di.comutator].Next().(byte)

		if b == 0x47 || b == 0xb8 { // Wait for first sync to start the DeInterleaver
			di.gotSync = true
		}

		if di.gotSync {
			di.outBuffer[di.outN] = b
			di.outN++

			if di.outN == dvbsFrameSize {
				di.outN = 0
				di.frameFifo.Add(di.outBuffer)
				di.outBuffer = make([]byte, dvbsFrameSize)
			}
		}

		// Move comutator
		di.comutator += 1
		di.comutator %= di.I
	}

	di.Unlock()
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
