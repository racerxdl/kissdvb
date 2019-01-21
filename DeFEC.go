package main

import (
	"github.com/OpenSatelliteProject/libsathelper"
	"log"
	"math/bits"
	"sync"
)

type DeFEC struct {
	sync.Mutex
	viterbi27        SatHelper.Viterbi27
	encodedSize      int
	encodedBuffer    []byte
	encodedBufferPos int
	decodedBuffer    [][]byte
	extraBits        []byte
	tmpBuffer        []byte

	lock        bool
	lockedFrame int
	bitErrors   []int
	frameReady  bool
}

func MakeDeFEC() *DeFEC {
	encodedSize := scanBits
	viterbiSize := encodedSize // + 6
	viterbi := SatHelper.NewViterbi27(viterbiSize)
	decodedBuffer := make([][]byte, 8)

	for i := 0; i < 8; i++ {
		decodedBuffer[i] = make([]byte, viterbi.DecodedSize())
	}

	encodedSize = viterbi.EncodedSize()

	return &DeFEC{
		encodedSize:      encodedSize,
		viterbi27:        viterbi, // Start with two frames
		encodedBuffer:    make([]byte, encodedSize),
		decodedBuffer:    decodedBuffer,
		encodedBufferPos: 0,
		extraBits:        make([]byte, 0),
		tmpBuffer:        make([]byte, encodedSize),
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
		fec.encodedBufferPos = 0
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
		copy(fec.tmpBuffer, fec.encodedBuffer)
		i := fec.lockedFrame
		if i > 0 {
			rotateSoftBuffer(fec.tmpBuffer, i, int(i/4) > 0)
		}
		fec.viterbi27.Decode(&fec.tmpBuffer[0], &fec.decodedBuffer[i][0])
		fec.bitErrors[i] = fec.viterbi27.GetBER()

		if !fec.syncPresentN(i) {
			// Lost lock
			log.Printf("Lost lock at %d!", i)
			fec.lock = false
		}
	}

	if !fec.lock {
		// Do all rotation decode
		for i := 0; i < 8; i++ {
			copy(fec.tmpBuffer, fec.encodedBuffer)
			// Rotate
			if i > 0 {
				rotateSoftBuffer(fec.tmpBuffer, i, int(i/4) > 0)
			}
			fec.viterbi27.Decode(&fec.tmpBuffer[0], &fec.decodedBuffer[i][0])
			fec.bitErrors[i] = fec.viterbi27.GetBER()
		}
	}

	return true
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

func (fec *DeFEC) ResetBuffer() {
	fec.encodedBufferPos = 0 // Reset encoded buffer
}

func (fec *DeFEC) syncPresentN(n int) bool {
	buff := fec.decodedBuffer[n]

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
		return fec.decodedBuffer[fec.lockedFrame]
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
		return fec.bitErrors[fec.lockedFrame]
	}

	return -1
}
