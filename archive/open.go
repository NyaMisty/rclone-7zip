package archive

import (
	"github.com/itchio/sevenzip-go/sz"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	sevenzip "github.com/itchio/sevenzip-go/sz"
)

var lib *sevenzip.Lib

func InitSevenZip() {
	var err error
	lib, err = sevenzip.NewLib()
	must(err)
	log.Infof("Initialized 7-zip %s...", lib.GetVersion())
}

type mcs struct {
	FirstName  string
	CurVolName string
	f          *os.File
}

func (m *mcs) GetFirstVolumeName() string {
	log.Tracef("mcs GetFirstVolumeName()")
	m.MoveToVolume(m.FirstName)
	return m.FirstName
}

func (m *mcs) MoveToVolume(volumeName string) error {
	log.Tracef("mcs MoveToVolume(%s)", volumeName)
	var err error
	m.f.Close()
	m.f, err = os.OpenFile(volumeName, os.O_RDONLY, 0)
	if err != nil {
		log.Tracef("mcs MoveToVolume(%s), error: %v", volumeName, err)
		return err
	}
	m.CurVolName = volumeName
	return nil
}

func (m *mcs) GetCurrentVolumeSize() uint64 {
	log.Tracef("mcs GetCurrentVolumeSize()")
	info, err := m.f.Stat()
	if err != nil {
		return 0
	}
	return uint64(info.Size())
}

func (m *mcs) OpenCurrentVolumeStream() (*sz.InStream, error) {
	log.Tracef("mcs OpenCurrentVolumeStream()")
	ext := filepath.Ext(m.CurVolName)
	if ext != "" {
		ext = ext[1:]
	}

	f, err := os.OpenFile(m.CurVolName, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	return sz.NewInStream(f, ext, int64(m.GetCurrentVolumeSize()))
}

func OpenArchive(inPath string, password string) (a *sevenzip.Archive, itemCount int64) {
	ext := filepath.Ext(inPath)
	if ext != "" {
		ext = ext[1:]
	}
	log.Tracef("ext = %s", ext)
	mc, err := sz.NewMultiVolumeCallback(&mcs{FirstName: inPath})
	must(err)

	log.Debugf("Created multi volume handler (%s)...", inPath)

	log.Infof("Opening archive with (%v, password %s)...", mc, password)
	a, err = lib.OpenMultiVolumeArchive(mc, password, false)
	must(err)

	log.Infof("Opened archive: format is (%s)", a.GetArchiveFormat())

	itemCount, err = a.GetItemCount()
	must(err)
	log.Infof("Archive has %d items", itemCount)
	return
}

func ExtractArchive(a *sevenzip.Archive, itemCount int64, funcs sevenzip.ExtractCallbackFuncs) {
	var indices = make([]int64, itemCount)
	for i := 0; i < int(itemCount); i++ {
		indices[i] = int64(i)
	}

	ExtractArchiveEx(a, indices, funcs)
}

func ExtractArchiveEx(a *sevenzip.Archive, items []int64, funcs sevenzip.ExtractCallbackFuncs) {
	ec, err := sz.NewExtractCallback(funcs)
	must(err)
	defer ec.Free()

	err = a.ExtractSeveral(items, ec)
	must(err)

	errs := ec.Errors()
	if len(errs) > 0 {
		log.Warnf("There were %d errors during extraction:", len(errs))
		for _, err := range errs {
			log.Warnf("- %s", err.Error())
		}
	}
}

func must(err error) {
	if err != nil {
		log.Panic(err)
	}
}
