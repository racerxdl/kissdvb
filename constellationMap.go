package main

import "fmt"

func rotateByte90(b2 []byte) {
	x := b2[0]
	y := b2[1]

	b2[0] = 254 - y
	b2[1] = x
}

func rotateSoftBufferBytes(buffer []byte, n int, conj bool) {
	if len(buffer)%2 > 0 {
		fmt.Println("WARN: Non even buffer!!")
	}

	for i := 0; i < len(buffer)/2; i++ {
		// Sync Word
		for z := 0; z < n%4; z++ {
			rotateByte90(buffer[i*2:])
		}

		if conj {
			buffer[i*2+1] = 254 - buffer[i*2+1]
		}
	}
}
