package main

type Correlator struct {
	words          [][]byte
	correlation    []uint32
	tmpCorrelation []uint32
	position       []uint32

	wordNumber int
}

func MakeCorrelator(syncWords [][]byte) *Correlator {
	return &Correlator{
		words:          syncWords,
		wordNumber:     0,
		correlation:    make([]uint32, len(syncWords)),
		tmpCorrelation: make([]uint32, len(syncWords)),
		position:       make([]uint32, len(syncWords)),
	}
}

func (cr *Correlator) Reset() {
	for i := 0; i < len(cr.words); i++ {
		cr.correlation[i] = 0
		cr.tmpCorrelation[i] = 0
		cr.position[i] = 0
	}
}

func hardCorrelate(a, b byte) uint32 {
	if a >= 127 && b == 0 {
		return 1
	}

	if a < 127 && b == 255 {
		return 1
	}

	return 0
}

func (cr *Correlator) Correlate(data []byte) {
	maxSearch := len(data) - 16
	numWords := len(cr.words)
	cr.Reset()

	for i := 0; i < maxSearch; i++ {
		// Reset temporary values
		for n := 0; n < numWords; n++ {
			cr.tmpCorrelation[n] = 0
		}

		for k := 0; k < 16; k++ {
			for n := 0; n < numWords; n++ {
				cr.tmpCorrelation[n] += hardCorrelate(data[i+k], cr.words[n][k])
			}
		}

		for n := 0; n < numWords; n++ {
			if cr.tmpCorrelation[n] > cr.correlation[n] {
				cr.correlation[n] = cr.tmpCorrelation[n]
				cr.position[n] = uint32(i)
				cr.tmpCorrelation[n] = 0
			}
		}
	}

	corr := uint32(0)

	for n := 0; n < numWords; n++ {
		if cr.correlation[n] > corr {
			corr = cr.correlation[n]
			cr.wordNumber = n
		}
	}
}

func (cr *Correlator) GetHighestCorrelation() uint32 {
	return cr.correlation[cr.wordNumber]
}

func (cr *Correlator) GetHighestCorrelationPosition() uint32 {
	return cr.position[cr.wordNumber]
}

func (cr *Correlator) GetHighestCorrelationWordNumber() int {
	return cr.wordNumber
}
