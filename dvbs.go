package main

import (
	"github.com/racerxdl/segdsp/tools"
	"log"
	"math"
)

const mpegtsFrameSize = 188
const dvbsFrameSize = 204
const dvbsFrameBits = dvbsFrameSize * 8

var rotation90 = complex(tools.Cos(float32(math.Pi/2)), tools.Sin(float32(math.Pi/2)))

const scanPackets = 8
const scanBits = dvbsFrameBits * scanPackets

var derandomizerLut []byte

func init() {
	derandomizerLut = make([]byte, mpegtsFrameSize*scanPackets)
	derandomizerLut[0] = 0xFF

	st := uint16(169)

	for i := 1; i < mpegtsFrameSize*scanPackets; i++ {
		out := byte(0)
		for n := 0; n < 8; n++ {
			bit := ((uint(st) >> 13) ^ (uint(st) >> 14)) & 1
			out = byte((uint(out) << 1) | bit) // MSB first
			st = uint16((uint(st) << 1) | bit) // Feedback
		}

		if i%188 != 0 {
			derandomizerLut[i] = out
		} else {
			derandomizerLut[i] = 0x00 // Sync bytes are not xored
		}
	}
}

func DeRandomize(frame []byte) {
	if len(frame) != mpegtsFrameSize*scanPackets {
		log.Printf("Expected %d got %d for DeRandomize Size", mpegtsFrameSize*scanPackets, len(frame))
		return
	}

	for i := 0; i < len(frame); i++ {
		frame[i] ^= derandomizerLut[i]
	}
}
