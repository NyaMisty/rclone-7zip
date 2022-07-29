package utils

import (
	"bytes"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBetterCopy1(t *testing.T) {
	assert := assert.New(t)
	src := bytes.NewBuffer(make([]byte, 0, 256*1024*1024))
	_tmp := make([]byte, 1024)
	for i := 0; i < src.Cap(); i += 1024 {
		_tmp[0] = byte(i / 1024)
		src.Write(_tmp)
	}
	dst := bytes.NewBuffer(make([]byte, src.Cap()))
	fullLen := src.Len()
	n, err := BetterCopy(dst, src, 256*1024, nil)
	log.Infof("BetterCopy n: %v err: %v", n, err)
	assert.Equal(n, fullLen)
	assert.Nil(err)
}
