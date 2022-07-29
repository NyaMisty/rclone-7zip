package main

import (
	"fmt"
	"github.com/NyaMisty/rclone-7zip/archive"
	"github.com/NyaMisty/rclone-7zip/rclone_utils"
	"github.com/alecthomas/kong"
	log "github.com/sirupsen/logrus"
	"io"
)

var RunArg struct {
	Debug           bool   `name:"debug" env:"RC_DEBUG" help:"Enable verbose debug output"`
	RcAddr          string `name:"rcaddr" env:"RC_ADDR" default:"http://127.0.0.1:5572" help:"URL to Rclone RC server"`
	RcAuth          string `name:"rcauth" env:"RC_AUTH" default:"" help:"Auth username:password for Rclone RC"`
	MaxTransfer     int    `name:"transfers" default:"10" help:"Max running transfer"`
	ArchiveFile     string `arg:"" help:"Archive to upload"`
	ArchivePassword string `name:"password" help:"Archive password"`
	UploadPath      string `arg:"" help:"Upload Rclone path (fs:path)"`
	UploadBuffer    int64  `name:"buffer" default:"134217728" help:"Buffer for data transmission between Rclone"` // 128M
}

var g_rclone *rclone_utils.RcloneUtil

func initRclone() {
	g_rclone = &rclone_utils.RcloneUtil{
		RcloneMode:   "rc",
		RcloneRcAddr: RunArg.RcAddr,
		RcloneRcAuth: RunArg.RcAuth,
		MaxTransfer:  int64(RunArg.MaxTransfer),
	}
	g_rclone.Init()
	log.Infof("Initialized Rclone with arg: %v", g_rclone)
}

func main() {
	ctx := kong.Parse(&RunArg)
	log.SetFormatter(&log.TextFormatter{ForceColors: true})
	log.SetLevel(log.InfoLevel)
	if RunArg.Debug {
		log.SetLevel(log.DebugLevel)
	}

	log.Infof("Got command & env flags: %v", ctx)
	initRclone()

	archive.InitSevenZip()
	a, itemCount := archive.OpenArchive(RunArg.ArchiveFile, RunArg.ArchivePassword)

	// at first we need to process all items
	processItems := make([]int64, itemCount)
	for i := 0; i < int(itemCount); i++ {
		processItems[i] = int64(i)
	}

	type FailItemInfo struct {
		ItemId   int64
		ItemPath string
	}
	failedItems := make([]FailItemInfo, 0, 10)
	for i := 0; i < 10; i++ {
		log.Infof("Extracting all items (%d items), round %d", len(processItems), i)
		failedItems = make([]FailItemInfo, 0, 10)
		handler := &archive.SZExtractHandler{
			StreamFactory: func(archiveIndex int64, path string, size int64) (io.WriteCloser, error) {
				log.Infof("StreamFactory handler creating rcatSize(%s, %d)", path, size)
				return g_rclone.RcatSize(RunArg.UploadPath+"/"+path, size, RunArg.UploadBuffer, func(resp interface{}, err error) {
					if err != nil {
						log.Errorf("StreamFactory handler rcatSize(%s, %d) failed", path, size)
						failedItems = append(failedItems, FailItemInfo{archiveIndex, path})
					}
					log.Infof("StreamFactory handler rcatSize(%s, %d) finished, resp %v err %v", path, size, resp, err)
				})
			},
		}
		archive.ExtractArchiveEx(a, processItems, handler)
		g_rclone.WaitAllAsyncReq()
		if len(failedItems) == 0 {
			log.Infof("All items successfully extracted, exiting!")
			break
		} else {
			log.Warnf("Items failed: %v", failedItems)
		}
		// identify which item we need to re-process
		processItems = make([]int64, len(failedItems))
		for i := 0; i < len(failedItems); i++ {
			processItems[i] = failedItems[i].ItemId
		}
	}
	if len(failedItems) != 0 {
		panic(fmt.Sprintf("Still have %d items failed after retries: %v", len(failedItems), failedItems))
	}
}

func must(err error) {
	if err != nil {
		log.Panic(err)
	}
}
