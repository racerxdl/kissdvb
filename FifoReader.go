package main

import (
	"sync"
)

type FifoReader struct {
	internalBuffer []byte
	cond           *sync.Cond
}

func MakeFifoReader() *FifoReader {
	return &FifoReader{
		internalBuffer: make([]byte, 0),
		cond:           sync.NewCond(&sync.Mutex{}),
	}
}

func (fr *FifoReader) Read(p []byte) (n int, err error) {
	fr.cond.L.Lock()
	// Wait until enough data is available
	for len(fr.internalBuffer) < len(p) {
		fr.cond.Wait()
	}

	// Here we should have enough data.
	copy(p, fr.internalBuffer[:len(p)])
	fr.internalBuffer = fr.internalBuffer[len(p):]

	fr.cond.L.Unlock()

	return len(p), nil
}

func (fr *FifoReader) PutData(d []byte) {
	fr.cond.L.Lock()
	fr.internalBuffer = append(fr.internalBuffer, d...)
	fr.cond.Broadcast()
	fr.cond.L.Unlock()
}
