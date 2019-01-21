package main

func f2bSoft(v float32) byte {
	return byte(v + 127)
}

func rotateSoftBuffer(buffer []byte, n int, conj bool) {
	for i := 0; i < len(buffer)/2; i++ {
		// Sync Word
		b0 := int(buffer[i*2]) - 127
		b1 := int(buffer[i*2+1]) - 127

		c := complex(float32(b0), float32(b1))
		for z := 0; z < n%4; z++ {
			c *= rotation90
		}

		buffer[i*2] = f2bSoft(real(c))
		if conj {
			buffer[i*2+1] = f2bSoft(-imag(c))
		} else {
			buffer[i*2+1] = f2bSoft(imag(c))
		}
	}
}
