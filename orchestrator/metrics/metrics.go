package metrics

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	log "github.com/xlab/suplog"
)

func ReportFuncError(tags ...Tags) {
	fn := funcName()
	reportFunc(fn, "error", tags...)
}

func ReportClosureFuncError(name string, tags ...Tags) {
	reportFunc(name, "error", tags...)
}

func ReportFuncStatus(tags ...Tags) {
	fn := funcName()
	reportFunc(fn, "status", tags...)
}

func ReportClosureFuncStatus(name string, tags ...Tags) {
	reportFunc(name, "status", tags...)
}

func ReportFuncCall(tags ...Tags) {
	fn := funcName()
	reportFunc(fn, "called", tags...)
}

func ReportClosureFuncCall(name string, tags ...Tags) {
	reportFunc(name, "called", tags...)
}

func reportFunc(fn, action string, tags ...Tags) {
	clientMux.RLock()
	defer clientMux.RUnlock()
	if client == nil {
		return
	}
	tagArray := joinTags(tags...)
	tagArray = append(tagArray, "func_name:"+fn)
	client.Incr(fmt.Sprintf("func.%v", action), tagArray, 0.77)
}

type StopTimerFunc func()

func ReportFuncTiming(tags ...Tags) StopTimerFunc {
	clientMux.RLock()
	defer clientMux.RUnlock()
	if client == nil {
		return func() {}
	}
	t := time.Now()
	fn := funcName()
	tagArray := joinTags(tags...)
	tagArray = append(tagArray, "func_name:"+fn)

	fmt.Println(tagArray)

	doneC := make(chan struct{})
	go func(name string, start time.Time) {
		select {
		case <-doneC:
			return
		case <-time.NewTicker(config.StuckFunctionTimeout).C:
			clientMux.RLock()
			defer clientMux.RUnlock()

			err := fmt.Errorf("detected stuck function: %s stuck for %v", name, time.Since(start))
			log.WithError(err).Warningln("detected stuck function")
			client.Incr("func.stuck", tagArray, 1)

		}
	}(fn, t)

	return func() {
		d := time.Since(t)
		close(doneC)

		clientMux.RLock()
		defer clientMux.RUnlock()
		client.Timing("func.timing", d, tagArray, 1)
	}
}

func ReportClosureFuncTiming(name string, tags ...Tags) StopTimerFunc {
	clientMux.RLock()
	defer clientMux.RUnlock()
	if client == nil {
		return func() {}
	}
	t := time.Now()
	tagArray := joinTags(tags...)
	tagArray = append(tagArray, "func_name:"+name)

	doneC := make(chan struct{})
	go func(name string, start time.Time) {
		select {
		case <-doneC:
			return
		case <-time.NewTicker(config.StuckFunctionTimeout).C:
			clientMux.RLock()
			defer clientMux.RUnlock()

			err := fmt.Errorf("detected stuck function: %s stuck for %v", name, time.Since(start))
			log.WithError(err).Warningln("detected stuck function")
			client.Incr("func.stuck", tagArray, 1)

		}
	}(name, t)

	return func() {
		d := time.Since(t)
		close(doneC)

		clientMux.RLock()
		defer clientMux.RUnlock()
		client.Timing("func.timing", d, tagArray, 1)

	}
}

func funcName() string {
	pc, _, _, _ := runtime.Caller(2)
	fullName := runtime.FuncForPC(pc).Name()
	parts := strings.Split(fullName, "/")
	nameParts := strings.Split(parts[len(parts)-1], ".")
	return nameParts[len(nameParts)-1]
}

type Tags map[string]string

func (t Tags) With(k, v string) Tags {
	if t == nil || len(t) == 0 {
		return map[string]string{
			k: v,
		}
	}
	t[k] = v
	return t
}

func joinTags(tags ...Tags) []string {
	if len(tags) == 0 {
		return []string{}
	}
	tagArray := make([]string, len(tags))
	i := 0
	for k, v := range tags[0] {
		tag := fmt.Sprintf("%s:%s", k, v)
		tagArray[i] = tag
		i += 1
	}
	return tagArray
}
