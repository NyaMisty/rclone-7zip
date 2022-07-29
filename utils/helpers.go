package utils

import (
	"fmt"
	"io"
	"time"
)

var COPY_BUFFER_SIZE = 32 * 1024

// BetterCopy copies src to dst, just like io.Copy, but using two goroutine allowing for simultaneously read & write
// The working mechanism is:
// 		Reader -> chan []byte (buffered) -> Writer
// When reader finishes, this func can finish at once, or waiting writer to finish like io.Copy
// 		- if you supply writerFinishHandler, then it will return when reader EOF,
//			and send writer result to the handler instead
// 		- if you give a nil to the handler, it will wait until writer finished all chunks in chan
// With these feature, you can maximize the copy bandwidth
func BetterCopy(dst io.Writer, src io.Reader, bufferSize int, writerFinishHandler func(writerErr error)) (int, error) {
	// ringBuf := ringbuffer.New(bufferSize)
	readerChan := make(chan bool)
	writerChan := make(chan bool)
	copyChan := make(chan []byte, bufferSize/COPY_BUFFER_SIZE+1)
	var curErr error
	written := 0
	go func() {
		// Reader goroutine
		for {
			if curErr != nil {
				break
			}
			buf := make([]byte, COPY_BUFFER_SIZE)
			n, err := src.Read(buf)
			// log.Infof("Read: %v %v", n, err)
			//written += n (We count written in writer instead)
			copyChan <- buf[:n]
			// log.Infof("ReadSent")
			if err != nil {
				// read error
				curErr = err
				break
			}

		}
		readerChan <- true
	}()

	go func() {
		// Writer goroutine
	out:
		for {
			var buf []byte
			select {
			case _buf := <-copyChan:
				// log.Infof("Write buf%v", len(_buf))
				buf = _buf
			case <-time.After(2 * time.Second):
				// log.Infof("(Write recv timeout)")
				if curErr != nil {
					// no more data in chan & error exists
					break out
				}
			}

			n, err := dst.Write(buf)
			// log.Infof("Wrote buf")
			if n < len(buf) && err == nil {
				// short write
				curErr = fmt.Errorf("short write")
				break
			}
			written += n
			if err != nil {
				// write error
				curErr = err
				break
			}

		}
		writerChan <- true
	}()

	var finalErr error
	handleReaderExit := func() {
		// wait for Reader exit signal
		<-readerChan
		close(readerChan)
		finalErr = curErr
		if curErr == io.EOF {
			finalErr = nil
		}
	}

	handleWriterExit := func() {
		// wait for Writer exit signal
		<-writerChan
		close(writerChan)
		finalErr = curErr
		if curErr == io.EOF {
			finalErr = nil
		}
	}

	handleReaderExit()
	if writerFinishHandler != nil {
		go func() {
			// if writerFinishHandler supplied, handle the writer exit there instead
			handleWriterExit()
			writerFinishHandler(finalErr)
		}()
		return written, finalErr
	}
	handleWriterExit()
	return written, finalErr
}
