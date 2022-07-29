package archive

import (
	"fmt"
	"github.com/itchio/sevenzip-go/sz"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io"
	"path/filepath"
	"strings"
	"time"
)

type SZExtractHandler struct {
	StreamFactory func(archiveIndex int64, path string, size int64) (io.WriteCloser, error)
}

func (e *SZExtractHandler) GetStream(item *sz.Item) (*sz.OutStream, error) {
	item.GetArchiveIndex()
	propPath, ok := item.GetStringProperty(sz.PidPath)
	if !ok {
		return nil, errors.New("could not get item path")
	}

	outPath := filepath.ToSlash(propPath)
	// Remove illegal character for windows paths, see
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx
	for i := byte(0); i <= 31; i++ {
		outPath = strings.Replace(outPath, string([]byte{i}), "_", -1)
	}

	isDir, _ := item.GetBoolProperty(sz.PidIsDir)
	if isDir {
		log.Debugf("Ignoring Directory %s", outPath)

		// is a dir, just skip it
		return nil, nil
	}
	log.Infof("==> Extracting %d: %s", item.GetArchiveIndex(), outPath)

	if attrib, ok := item.GetUInt64Property(sz.PidAttrib); ok {
		log.Debugf("==> Attrib       %08x", attrib)
	}
	var size uint64
	if _size, ok := item.GetUInt64Property(sz.PidSize); ok {
		size = _size
		log.Infof("==> Size		%12d", size)
	}
	if attrib, ok := item.GetUInt64Property(sz.PidPosixAttrib); ok {
		log.Debugf("==> Posix Attrib %08x", attrib)
	}
	if symlink, ok := item.GetStringProperty(sz.PidSymLink); ok {
		log.Debugf("==> Symlink dest: %s", symlink)
	}

	of, err := e.StreamFactory(item.GetArchiveIndex(), outPath, int64(size))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	os, err := sz.NewOutStream(of)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return os, nil
}

func ByteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}

var lastReport time.Time

func (e *SZExtractHandler) SetProgress(complete int64, total int64) {
	if time.Now().Sub(lastReport) > time.Second*3 {
		log.Warnf("Total Progress: %s / %s %.2f",
			ByteCountIEC(complete),
			ByteCountIEC(total),
			float64(complete)/float64(total)*100,
		)
		lastReport = time.Now()
	}
}
