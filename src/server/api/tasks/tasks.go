package tasks

import (
	"context"
	"time"
)

var taskCtx context.Context = context.Background()
var taskCancelCtx context.CancelFunc

func Start() {
	ctx, cancel := context.WithCancel(taskCtx)
	taskCtx = ctx
	taskCancelCtx = cancel

	CheckEmotesPopularity(taskCtx)
}

func Cleanup() {
	taskCancelCtx()
	time.Sleep(time.Millisecond * 500)
}
