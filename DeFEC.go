package main

import (
    "github.com/OpenSatelliteProject/libsathelper"
    "log"
    "sync"
)

type DeFEC struct {
    sync.Mutex
    viterbi27 SatHelper.Viterbi27
    encodedSize int
    encodedBuffer []byte
    encodedBufferPos int
    decodedBuffer [][]byte
    extraBits []byte
    tmpBuffer []byte

    lock bool
    lockedFrame int
}

func MakeDeFEC() *DeFEC {
    encodedSize := scanBits
    viterbiSize := encodedSize + 2
    viterbi := SatHelper.NewViterbi27(viterbiSize)
    decodedBuffer := make([][]byte, 8)

    for i := 0; i < 8; i++ {
        decodedBuffer[i] = make([]byte, viterbi.DecodedSize())
    }

    encodedSize = viterbi.EncodedSize()

    return &DeFEC{
        encodedSize: encodedSize,
        viterbi27: viterbi, // Start with two frames
        encodedBuffer: make([]byte, encodedSize),
        decodedBuffer: decodedBuffer,
        encodedBufferPos: 0,
        extraBits: make([]byte, 0),
        tmpBuffer: make([]byte, encodedSize),
        lock: false,
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

    if fec.encodedBufferPos != fec.encodedSize && len(fec.extraBits) > 0 {
        fec.addBitsToBuffer()
    }

    if fec.encodedBufferPos != fec.encodedSize {
        return false
    }

    if fec.lock { // We had already locked last frame, let's just retry that
        copy(fec.tmpBuffer, fec.encodedBuffer)
        i := fec.lockedFrame
        if i > 0 {
            rotateSoftBuffer(fec.tmpBuffer, i, int(i / 4) > 0)
        }
        fec.viterbi27.Decode(&fec.tmpBuffer[0], &fec.decodedBuffer[i][0])
        if !fec.syncPresentN(i) {
            // Lost lock
            log.Println("Lost lock!")
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
            //_ = ioutil.WriteFile(fmt.Sprintf("tmpfile%d", i), fec.decodedBuffer[i][:scanBits / 8], 0770)
        }
    }

    return true
}

func (fec *DeFEC) TryFindSync() int {
    for fec.UpdateOut() {
        dvbSync := fec.syncPresent()
        if dvbSync != -1 {
            //log.Printf("Found sync %d!\n", dvbSync)
            //_ = ioutil.WriteFile("tmpfile", fec.decodedBuffer[dvbSync], 0770)
            return dvbSync
        }
        // Try shiffting one bit
        fec.shiftOneBit()
    }

    return -1
}

func (fec *DeFEC) syncPresentN(n int) bool {
    buff := fec.decodedBuffer[n]

    return buff[0] == 0xB8 && buff[204] == 0x47 && buff[408] == 0x47
}

func (fec *DeFEC) syncPresent() int {
    // Check All Constellation Rotations
    for i := 0; i < 8; i++ {
        if fec.syncPresentN(i) {
            fec.lock = true
            fec.lockedFrame = i
            return i
        }
    }

    return -1
}

func (fec *DeFEC) GetLockedFrame() []byte {
    if fec.lock {
        return fec.decodedBuffer[fec.lockedFrame]
    }

    return nil
}

func (fec *DeFEC) IsLocked() bool {
    return fec.lock
}