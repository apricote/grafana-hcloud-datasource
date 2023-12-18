package logutil

import (
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"io"
)

func NewDebugWriter(logger log.Logger) io.Writer {
	return writer{logger}
}

type writer struct {
	logger log.Logger
}

func (dw writer) Write(p []byte) (n int, err error) {
	dw.logger.Info(string(p))
	return len(p), nil
}
