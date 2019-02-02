package main

import (
	"fmt"
	"github.com/OpenSatelliteProject/libsathelper"
	"log"
	"math/bits"
	"sync"
)

const numLastFrameBits = 32
const numLastFrameBitsInBytes = numLastFrameBits / 8

type DeFEC struct {
	sync.Mutex
	viterbi27        []SatHelper.Viterbi27
	encodedSize      int
	encodedBuffer    []byte
	encodedBufferPos int
	decodedBuffer    [][]byte
	extraBits        []byte
	tmpBuffers       [][]byte

	lock        bool
	lockedFrame int
	bitErrors   []int
	frameReady  bool
}

func MakeDeFEC() *DeFEC {
	encodedSize := (scanBits + numLastFrameBits) * 2
	viterbi := make([]SatHelper.Viterbi27, 8)
	decodedBuffer := make([][]byte, 8)
	tmpBuffers := make([][]byte, 8)

	for i := 0; i < 8; i++ {
		viterbi[i] = SatHelper.NewViterbi27(encodedSize / 2)
		decodedBuffer[i] = make([]byte, viterbi[0].DecodedSize())
		tmpBuffers[i] = make([]byte, encodedSize)
	}

	encodedBuffer := make([]byte, encodedSize)

	for i := 0; i < encodedSize; i++ {
		encodedBuffer[i] = 127
	}

	return &DeFEC{
		encodedSize:      encodedSize,
		viterbi27:        viterbi,
		encodedBuffer:    encodedBuffer,
		decodedBuffer:    decodedBuffer,
		encodedBufferPos: numLastFrameBits * 2,
		extraBits:        make([]byte, 0),
		tmpBuffers:       tmpBuffers,
		lock:             false,
		bitErrors:        make([]int, 8),
	}
}

func (fec *DeFEC) PutSoftBits(bits []byte) {
	fec.Lock()
	fec.extraBits = append(fec.extraBits, bits...)
	fec.addBitsToBuffer()
	fec.Unlock()
}

func (fec *DeFEC) shiftOneBit() {
	fec.shiftNBits(1)
}

func (fec *DeFEC) shiftNBits(n int) {
	if n == 0 {
		return
	}

	fec.Lock()

	if fec.encodedBufferPos > n {
		copy(fec.encodedBuffer, fec.encodedBuffer[n:])
		fec.encodedBufferPos -= n
	} else {
		fec.encodedBufferPos = numLastFrameBits * 2
		for i := 0; i < numLastFrameBits*2; i++ {
			fec.encodedBuffer[i] = 127
		}
	}

	fec.addBitsToBuffer()

	fec.Unlock()
}

func (fec *DeFEC) addBitsToBuffer() {
	bitsToAdd := len(fec.extraBits)
	missingBytes := fec.encodedSize - fec.encodedBufferPos
	if bitsToAdd > missingBytes {
		bitsToAdd = missingBytes
	}

	if bitsToAdd > 0 {
		copy(fec.encodedBuffer[fec.encodedBufferPos:], fec.extraBits[:bitsToAdd])
		fec.extraBits = fec.extraBits[bitsToAdd:]
		fec.encodedBufferPos += bitsToAdd
	}
}

func (fec *DeFEC) fillTmpBuffers() {
	for i := 0; i < 8; i++ {
		copy(fec.tmpBuffers[i], fec.encodedBuffer)
	}
}

func (fec *DeFEC) UpdateOut() bool {
	fec.Lock()
	defer fec.Unlock()

	if fec.encodedBufferPos != fec.encodedSize && len(fec.extraBits) > 0 { // Add remaining bits if needed
		fec.addBitsToBuffer()
	}

	if fec.encodedBufferPos != fec.encodedSize { // If not enough bits in the buffer, break
		return false
	}

	if fec.lock { // We had already locked last frame, let's just retry that
		fec.fillTmpBuffers()
		i := fec.lockedFrame
		if i > 0 {
			rotateSoftBufferBytes(fec.tmpBuffers[i], i, int(i/4) > 0)
		}
		fec.viterbi27[i].Decode(&fec.tmpBuffers[i][0], &fec.decodedBuffer[i][0])
		fec.bitErrors[i] = fec.viterbi27[i].GetBER()

		if !fec.syncPresentN(i) {
			// Lost lock
			log.Printf("Lost lock at %d!", i)
			fec.lock = false
		}
	}

	if !fec.lock {
		// Do all rotation decode
		fec.updateViterbis()
	}

	return true
}

func (fec *DeFEC) updateViterbis() {
	// Update all viterbis in parallel
	wg := sync.WaitGroup{}
	wg.Add(8)

	fec.fillTmpBuffers()
	for i := 0; i < 8; i++ {
		go func(n int) {
			// Rotate
			if n > 0 {
				rotateSoftBufferBytes(fec.tmpBuffers[n], n, int(n/4) > 0)
			}
			fec.viterbi27[n].Decode(&fec.tmpBuffers[n][0], &fec.decodedBuffer[n][0])
			fec.bitErrors[n] = fec.viterbi27[n].GetBER()
			wg.Done()
		}(i)
	}

	wg.Wait()
}

//Not working, not sure why
func (fec *DeFEC) UpdateLockedFrame() {
	if fec.lockedFrame != 0 {
		fmt.Printf("Updating locked frame from %d to 0\n", fec.lockedFrame)
		// Re-add everything to extra bits
		fec.extraBits = append(fec.encodedBuffer[:fec.encodedBufferPos], fec.extraBits...)
		fec.encodedBufferPos = 0
		// Rotate
		rotateSoftBufferBytes(fec.extraBits, fec.lockedFrame, int(fec.lockedFrame/4) > 0)
		// Re-add everything
		fec.addBitsToBuffer()
		fec.lockedFrame = 0
	}
}

func (fec *DeFEC) TryFindSync() int {
	for fec.UpdateOut() {
		dvbSync := fec.syncPresent()
		if dvbSync != -1 {
			if fec.lock == false {
				log.Printf("Got lock at %d\n", dvbSync)
			}
			fec.lock = true
			fec.lockedFrame = dvbSync
			fec.frameReady = true
			fec.ResetBuffer()
			return dvbSync
		}
		// Try shiffting one bit
		fec.shiftOneBit()
	}

	return -1
}

func (fec *DeFEC) fillLastBits() {
	// Shift the end to start
	copy(fec.encodedBuffer[:numLastFrameBits*2], fec.encodedBuffer[fec.encodedSize-numLastFrameBits*2:])
}

func (fec *DeFEC) ResetBuffer() {
	fec.encodedBufferPos = numLastFrameBits * 2 // Reset encoded buffer
	fec.fillLastBits()
}

func (fec *DeFEC) syncPresentN(n int) bool {
	buff := fec.decodedBuffer[n][numLastFrameBitsInBytes:]

	// Soft Sync Present

	v := 0
	v += bits.OnesCount8(buff[0] ^ 0xB8)
	v += bits.OnesCount8(buff[204] ^ 0x47)
	v += bits.OnesCount8(buff[408] ^ 0x47)
	v += bits.OnesCount8(buff[612] ^ 0x47)
	v += bits.OnesCount8(buff[816] ^ 0x47)

	return v < 5
}

func (fec *DeFEC) syncPresent() int {
	// Check All Constellation Rotations
	for i := 0; i < 8; i++ {
		if fec.syncPresentN(i) {
			return i
		}
	}

	return -1
}

func (fec *DeFEC) GetLockedFrame() []byte {
	fec.Lock()
	defer fec.Unlock()

	if fec.lock && fec.frameReady {
		fec.frameReady = false
		return fec.decodedBuffer[fec.lockedFrame][numLastFrameBitsInBytes:]
	}

	return nil
}

func (fec *DeFEC) IsFrameReady() bool {
	return fec.frameReady
}

func (fec *DeFEC) IsLocked() bool {
	return fec.lock
}

func (fec *DeFEC) GetBER() int {
	fec.Lock()
	defer fec.Unlock()

	if fec.lock {
		e := fec.bitErrors[fec.lockedFrame] - numLastFrameBits
		if e < 0 {
			return 0
		}
		return e
	}

	return -1
}
