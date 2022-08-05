package rclone_utils

import (
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	"strings"
	"sync/atomic"
	"time"
)

type RcloneUtil struct {
	RcloneMode   string // "rc" / "command"
	RcloneRcAddr string // "http://server:port"
	RcloneRcAuth string // "user:password"

	MaxTransfer int64

	maxTransferSem *semaphore.Weighted

	Client *resty.Client
}

func (r *RcloneUtil) Init() {
	auth := strings.Split(r.RcloneRcAddr, ":")
	cli := resty.New().
		SetBaseURL(r.RcloneRcAddr)
	if r.RcloneRcAuth != "" {
		cli = cli.
			SetBasicAuth(auth[0], auth[1])
	}
	r.Client = cli

	r.maxTransferSem = semaphore.NewWeighted(r.MaxTransfer)
}

type RcloneRCResponse struct {
	HTTPResponse *resty.Response
	AsyncFinish  chan bool
}

var g_asyncThreads int32

func (r *RcloneUtil) WaitAllAsyncReq() {
	for {
		curTh := atomic.LoadInt32(&g_asyncThreads)
		if curTh == 0 {
			log.Infof("All Async Job finished!")
			return
		} else {
			log.Infof("Still %d jobs running...", curTh)
		}
		time.Sleep(5 * time.Second)
	}
}

func (r *RcloneUtil) _doRcReq(opName string, body map[string]interface{}, asyncCallback func(interface{}, error)) (*RcloneRCResponse, error) {
	isAsync := asyncCallback != nil

	R := r.Client.R()
	_body := make(map[string]interface{})
	if body != nil {
		for k, v := range body {
			_body[k] = v
		}
	}

	if isAsync {
		_body["_async"] = true
	}
	R = R.
		SetBody(_body)

	resp, err := R.Post(opName)
	if err != nil {
		return nil, err
	}
	//log.Debugf("Raw resp: str:%v json:%v hdr:%v", resp.String(), resp.Result(), resp.Header())
	ret := &RcloneRCResponse{
		HTTPResponse: resp,
		AsyncFinish:  make(chan bool, 1),
	}
	if isAsync {
		// asyncCallback is always called unless err
		type AsyncJobRet struct {
			Jobid int `json:"jobid"`
		}
		asyncJobRet := AsyncJobRet{}
		err = json.Unmarshal(resp.Body(), &asyncJobRet)
		log.Debugf("asyncHandler new job: %v", asyncJobRet)
		if err != nil || asyncJobRet.Jobid == 0 {
			return nil, fmt.Errorf("failed to get returned jobid, server resp: %v, err: %v", resp.String(), err)
		}
		jobId := asyncJobRet.Jobid
		go func() {
			atomic.AddInt32(&g_asyncThreads, 1)
			defer func() {
				atomic.AddInt32(&g_asyncThreads, -1)
				ret.AsyncFinish <- true
			}()
			for {
				time.Sleep(5 * time.Second)
				ret, err := r._doRcReq("job/status", map[string]interface{}{"jobid": jobId}, nil)
				if err != nil {
					continue
				}
				resp := ret.HTTPResponse
				var jobStatusRet struct {
					Error    string `json:"error"`
					Finished bool   `json:"finished"`
				}

				//log.Debugf("asyncJob Status: %d %s", resp.StatusCode(), resp.String())

				err = json.Unmarshal(resp.Body(), &jobStatusRet)
				if err != nil || jobStatusRet.Error != "" {
					log.Warnf("asyncHandler jobStatus(%d) error: %v", jobId, resp.String())
					asyncCallback(nil, fmt.Errorf("asyncJob job error, server resp: %v", resp.String()))
					return
				}
				if jobStatusRet.Finished {
					log.Debugf("asyncHandler job finished %d", jobId)
					asyncCallback(resp, nil)
					return
				}
			}
		}()
	}
	return ret, nil
}
