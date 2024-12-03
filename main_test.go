package main

import (
	"io"

	"github.com/google/logger"
)

func init() {
	logger.Init("transcode-factory", true, true, io.Discard)
}
