package h264

type AudioFrame struct {
	Samples    []float32
	SampleRate float64
}

func MakeAudioFrame(samples []float32, sampleRate float64) *AudioFrame {
	return &AudioFrame{
		Samples:    samples,
		SampleRate: sampleRate,
	}
}

type AudioParams struct {
	SampleRate       float64
	SamplesPerBuffer int
}
