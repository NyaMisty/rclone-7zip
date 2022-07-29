package utils

import (
	"fmt"
	"io"
	"time"
)

var COPY_BUFFER_SIZE = 32 * 1024

func BetterCopy(dst io.Writer, src io.Reader, bufferSize int, writerFinishHandler func(writerErr error)) (int, error) {
	// ringBuf := ringbuffer.New(bufferSize)
	readerChan := make(chan bool)
	writerChan := make(chan bool)
	copyChan := make(chan []byte, bufferSize/COPY_BUFFER_SIZE+1)
	var finalErr error
	written := 0
	go func() {
		for {
			if finalErr != nil {
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
				finalErr = err
				break
			}

		}
		readerChan <- true
	}()

	go func() {
	out:
		for {
			var buf []byte
			select {
			case _buf := <-copyChan:
				// log.Infof("Write buf%v", len(_buf))
				buf = _buf
			case <-time.After(2 * time.Second):
				// log.Infof("(Write recv timeout)")
				if finalErr != nil {
					// no more data in chan & error exists
					break out
				}
			}

			n, err := dst.Write(buf)
			// log.Infof("Wrote buf")
			if n < len(buf) && err == nil {
				// short write
				finalErr = fmt.Errorf("short write")
				break
			}
			written += n
			if err != nil {
				// write error
				finalErr = err
				break
			}

		}
		writerChan <- true
	}()

	handleReaderExit := func() {
		<-readerChan
		if finalErr == io.EOF {
			finalErr = nil
		}
	}

	handleWriterExit := func() {
		<-writerChan
		if finalErr == io.EOF {
			finalErr = nil
		}
	}

	handleReaderExit()
	if writerFinishHandler != nil {
		go func() {
			handleWriterExit()
			writerFinishHandler(finalErr)
		}()
		return written, finalErr
	}
	handleWriterExit()
	return written, finalErr
}
