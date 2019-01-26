package h264

/*
   #include "h264.h"
*/
import "C"
import (
	"errors"
	"unsafe"
)

type AACDecoder struct {
	m         C.aacdec_t
	tmpBuffer []float32
}

func NewAACDecoder() (m *AACDecoder, err error) {
	m = &AACDecoder{
		tmpBuffer: make([]float32, 1024*1024),
	}
	r := C.aacdec_new(&m.m)

	if int(r) < 0 {
		err = errors.New("open codec failed")
	}

	return
}

func (m *AACDecoder) SendPacket(packet []byte) int {
	r := C.aacdec_sendpacket(
		&m.m,
		(*C.uint8_t)(unsafe.Pointer(&packet[0])),
		(C.int)(len(packet)))

	return int(r)
}

func (m *AACDecoder) GetFrame() (af *AudioFrame, err error) {
	//runtime.LockOSThread()
	C.aacdec_recvpacket(&m.m, (*C.float)(unsafe.Pointer(&m.tmpBuffer[0])), (C.int)(len(m.tmpBuffer)*4))
	//runtime.UnlockOSThread()

	if m.m.got == 0 {
		err = errors.New("no audio")
		return
	}
	m.m.got = 0

	f := m.m.f

	//log.Printf("Line Size: %d\n", f.linesize[0] / f.channels)

	//log.Printf("Num Samples: %d\n", f.nb_samples)
	//log.Printf("Num Channels: %d\n", f.channels)
	//log.Printf("Sample Rate: %d\n", f.sample_rate)

	samples := make([]float32, m.m.f.nb_samples)

	copy(samples, m.tmpBuffer)
	af = MakeAudioFrame(samples, float64(f.sample_rate))

	return
}
