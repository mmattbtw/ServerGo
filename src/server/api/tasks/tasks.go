package tasks

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

var taskCtx context.Context = context.Background()
var taskCancelCtx context.CancelFunc

func Start() {
	ctx, cancel := context.WithCancel(taskCtx)
	taskCtx = ctx
	taskCancelCtx = cancel

	if err := CheckEmotesPopularity(taskCtx); err != nil {
		logrus.WithError(err).Error("failed to check popularity")
	}
}

func Cleanup() {
	taskCancelCtx()
	time.Sleep(time.Millisecond * 500)
}
