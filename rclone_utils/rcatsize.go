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
)

func (r *RcloneUtil) RcatSize(path string, size int64, bufferSize int64, asyncCallback func(resp interface{}, err error)) (writer io.WriteCloser, err error) {
	if r.RcloneMode == "rc" {
		_fifoTmpFile, err := ioutil.TempFile("", "rcatsize")
		if err != nil {
			return nil, err
		}
		fifoTmpPath := _fifoTmpFile.Name()
		_fifoTmpFile.Close()
		os.Remove(fifoTmpPath)
		err = unix.Mkfifo(fifoTmpPath, 0o666)
		if err != nil {
			log.Errorf("Failed to create fifo %s (%v)", _fifoTmpFile, err)
			return nil, err
		}
		log.Debugf("Created fifo %s (%v)", fifoTmpPath, err)
		reader, writer := io.Pipe()

		pathSplitTmp := strings.SplitN(path, ":", 2)
		dstFs := "/"
		dstRemote := path
		if len(pathSplitTmp) == 2 {
			dstFs = pathSplitTmp[0] + ":"
			dstRemote = pathSplitTmp[1]
		}
		log.Debugf("rcatSize dstFs: %v dstRemote: %v", dstFs, dstRemote)

		cleanup := func() {
			writer.Close()
			reader.Close()
			os.Remove(fifoTmpPath)
		}

		go func() {
			_, err = r._doRcReq("operations/rcatsize", map[string]interface{}{
				"type":   "fifo",
				"addr":   fifoTmpPath,
				"size":   size,
				"fs":     dstFs,
				"remote": dstRemote,
			}, asyncCallback)

			r.maxTransferSem.Acquire(context.Background(), 1)
			defer r.maxTransferSem.Release(1)

			// this open call will block until rclone connects
			fifoWriter, err := os.OpenFile(fifoTmpPath, os.O_WRONLY, os.ModeNamedPipe)
			if err != nil {
				//return nil, err
				log.Errorf("Failed to open fifo file, err: %v", err)
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
				cleanup()
			})
			log.Debugf("RcatSize ioCopy reader finished n: %d, err: %v", n, err)
		}()
		if err != nil {
			asyncCallback(nil, err)
			cleanup()
			return nil, err
		}

		return writer, nil
	} else {
		return nil, fmt.Errorf("rclone mode %v not implemented", r.RcloneMode)
	}
}
