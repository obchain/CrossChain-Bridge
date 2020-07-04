package worker

import (
	"time"

	"github.com/anyswap/CrossChain-Bridge/log"
)

var (
	maxRecallLifetime       = int64(10 * 24 * 3600)
	restIntervalInRecallJob = 3 * time.Second

	maxRetryLifetime       = int64(10 * 24 * 3600)
	restIntervalInRetryJob = 3 * time.Second

	maxVerifyLifetime       = int64(7 * 24 * 3600)
	restIntervalInVerifyJob = 3 * time.Second

	maxDoSwapLifetime       = int64(7 * 24 * 3600)
	restIntervalInDoSwapJob = 3 * time.Second

	maxStableLifetime       = int64(7 * 24 * 3600)
	restIntervalInStableJob = 3 * time.Second
)

func now() int64 {
	return time.Now().Unix()
}

func logWorker(job, subject string, context ...interface{}) {
	log.Info("["+job+"] "+subject, context...)
}

func logWorkerError(job, subject string, err error, context ...interface{}) {
	fields := []interface{}{"err", err}
	fields = append(fields, context...)
	log.Error("["+job+"] "+subject, fields...)
}

func logWorkerTrace(job, subject string, context ...interface{}) {
	log.Trace("["+job+"] "+subject, context...)
}

func getSepTimeInFind(dist int64) int64 {
	nowTime := now()
	if nowTime > dist {
		return nowTime - dist
	}
	return 0
}

func restInJob(duration time.Duration) {
	time.Sleep(duration)
}

func getPassedTimeSince(startTime int64) int64 {
	nowTime := now()
	if nowTime > startTime {
		return nowTime - startTime
	}
	return 0
}
