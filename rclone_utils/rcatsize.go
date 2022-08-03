package rclone_utils

import (
	"context"
	"fmt"
	"github.com/NyaMisty/rclone-7zip/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

func (r *RcloneUtil) RcatSize(path string, size int64, modTime time.Time, bufferSize int64, asyncCallback func(resp interface{}, err error)) (writer io.WriteCloser, err error) {
	// --- The Process of RcatSize
	// 0. Create pipe writer and return to the caller for transfer
	// 1. Create FIFO
	// 2. Prepare arguments for the rclone RC call, Do the RC call
	// 3. Open FIFO and start the transfer
	// --- Performance of RcatSize
	// In the above process:
	//		No0 rely on nothing
	//		No2 and No3 rely on No1
	//		No3's os.OpenFile will wait until rclone connects and read the FIFO
	// so we make:
	//		No0 in the caller goroutine to return ASAP
	//		No1 & No3 in goroutine1 to create FIFO and wait for copy
	//		No2 in goroutine2 to request RC
	// --- Ratelimiting of RcatSize
	// We also have to limit the transfer number to avoid memory exhaust
	// In our case, there're several blocking point, which we can actually inject semaphore code there
	// 1. at beginning & ending of goroutine1
	// 2. at copy beginning & ending
	// 3. at RC async job beginning & ending
	// the actual transfer is happening in Rclone, so we must choose 3 to avoid buffer stacks up in Rclone side

	reader, writer := io.Pipe()

	if r.RcloneMode == "rc" {
		go func() {
			// goroutine 1
			//r.maxTransferSem.Acquire(context.Background(), 1)
			//defer r.maxTransferSem.Release(1)

			// Create FIFO
			_fifoTmpFile, err := ioutil.TempFile("", "rcatsize")
			if err != nil {
				asyncCallback(nil, err)
				return
			}
			fifoTmpPath := _fifoTmpFile.Name()
			_fifoTmpFile.Close()
			os.Remove(fifoTmpPath)
			err = unix.Mkfifo(fifoTmpPath, 0o666)
			if err != nil {
				log.Errorf("Failed to create fifo %s (%v)", _fifoTmpFile, err)
				asyncCallback(nil, err)
				return
			}
			log.Debugf("Created fifo %s (%v)", fifoTmpPath, err)

			var fifoWriter *os.File

			// 	stopAndCleanup will close all pipes and let the fifo shutdown,
			//		and in turn made _doRcReq finish & do the callback
			// 		this function is safe to be called multiple times
			stopAndCleanup := func() {
				log.Debugf("Cleanup for %s, fifo %s!", path, fifoTmpPath)
				writer.Close()
				reader.Close()
				if fifoWriter != nil {
					fifoWriter.Close()
				}
				os.Remove(fifoTmpPath)
			}

			go func() {
				pathSplitTmp := strings.SplitN(path, ":", 2)
				dstFs := "/"
				dstRemote := path
				if len(pathSplitTmp) == 2 {
					dstFs = pathSplitTmp[0] + ":"
					dstRemote = pathSplitTmp[1]
				}
				log.Debugf("rcatSize dstFs: %v dstRemote: %v", dstFs, dstRemote)

				r.maxTransferSem.Acquire(context.Background(), 1)
				asyncCallbackWrap := func(resp interface{}, err error) {
					stopAndCleanup() // here we must close the fifoWriter, even if we may have already called it
					// or BetterCopy may be stuck at Write call and never finish
					defer r.maxTransferSem.Release(1)
					asyncCallback(resp, err)
				}

				_, err = r._doRcReq("operations/rcatsize", map[string]interface{}{
					"type":    "fifo",
					"addr":    fifoTmpPath,
					"size":    size,
					"modtime": modTime.Format(time.RFC3339),
					"fs":      dstFs,
					"remote":  dstRemote,
				}, asyncCallbackWrap)
				if err != nil { // start RC async job rcatsize failed
					stopAndCleanup()
					asyncCallbackWrap(nil, err)
					return
				}
				// from here, asyncCallbackWrap is only called by _doRcReq
			}()

			// --- From here, when exiting, we must call stopAndCleanup

			// this open call will block until rclone connects
			fifoWriter, err = os.OpenFile(fifoTmpPath, os.O_WRONLY, os.ModeNamedPipe)
			if err != nil {
				log.Errorf("Failed to open fifo file, err: %v", err)
				stopAndCleanup() // stop the pipe writer, and let _doRcReq to call the callback
				return
			}
			log.Debugf("Rclone opened the fifo, start transmitting!")

			if bufferSize > size {
				bufferSize = size
			}
			n, err := utils.BetterCopy(fifoWriter, reader, int(bufferSize), func(writerErr error) {
				if writerErr != nil {
					log.Warnf("RcatSize writer error: %v", writerErr)
				}
				err = fifoWriter.Close()
				log.Debugf("RcatSize ioCopy writer finished, fifoClose err: %v", err)
				stopAndCleanup() // do stopAndCleanup here because all writer are exited
				// we should do so because rclone may still wait for our fifo to transfer data
			})
			log.Debugf("RcatSize ioCopy reader finished n: %d, err: %v", n, err)
			// we can't stopAndCleanup here as BetterCopy's writer may still pending
		}()

		return writer, nil
	} else {
		return nil, fmt.Errorf("rclone mode %v not implemented", r.RcloneMode)
	}
}
