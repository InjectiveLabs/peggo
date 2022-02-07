package metrics

import (
	"sync"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"
)

var client Statter
var clientMux = new(sync.RWMutex)
var config *StatterConfig

type StatterConfig struct {
	EnvName              string
	HostName             string
	StuckFunctionTimeout time.Duration
	MockingEnabled       bool
}

func (m *StatterConfig) BaseTags() []string {
	var baseTags []string

	if len(config.EnvName) > 0 {
		baseTags = append(baseTags, "env:" + config.EnvName)
	}
	if len(config.HostName) > 0 {
		baseTags = append(baseTags, "machine:" + config.HostName)
	}

	return baseTags
}

type Statter interface {
	Count(name string, value int64, tags []string, rate float64) error
	Incr(name string, tags []string, rate float64) error
	Decr(name string, tags []string, rate float64) error
	Gauge(name string, value float64, tags []string, rate float64) error
	Timing(name string, value time.Duration, tags []string, rate float64) error
	Histogram(name string, value float64, tags []string, rate float64) error
	Close() error
}

func Close() {
	clientMux.RLock()
	defer clientMux.RUnlock()
	if client == nil {
		return
	}
	client.Close()
}

func Disable() {
	config = checkConfig(nil)
	clientMux.Lock()
	client = newMockStatter(true)
	clientMux.Unlock()
}

func Init(addr string, prefix string, cfg *StatterConfig) error {
	config = checkConfig(cfg)
	if config.MockingEnabled {
		// init a mock statter instead of real statsd client
		clientMux.Lock()
		client = newMockStatter(false)
		clientMux.Unlock()
		return nil
	}

	statter, err := statsd.New(
		addr,
		statsd.WithNamespace("injective-trading-bot"),
		statsd.WithWriteTimeout(time.Duration(10) * time.Second),
		statsd.WithTags(config.BaseTags()),
	)

	if err != nil {
		err = errors.Wrap(err, "statsd init failed")
		return err
	}
	clientMux.Lock()
	client = statter
	clientMux.Unlock()
	return nil
}

func checkConfig(cfg *StatterConfig) *StatterConfig {
	if cfg == nil {
		cfg = &StatterConfig{}
	}
	if cfg.StuckFunctionTimeout < time.Second {
		cfg.StuckFunctionTimeout = 5 * time.Minute
	}
	if len(cfg.EnvName) == 0 {
		cfg.EnvName = "local"
	}
	return cfg
}

func errHandler(err error) {
	log.WithError(err).Errorln("statsd error")
}

func newMockStatter(noop bool) Statter {
	return &mockStatter{
		noop: noop,
		fields: log.Fields{
			"module": "mock_statter",
		},
	}
}

type mockStatter struct {
	fields log.Fields
	noop   bool
}

func (s *mockStatter) Count(name string, value int64, tags []string, rate float64) error {
	if s.noop {
		return nil
	}
	log.WithFields(log.WithFn(s.fields)).Debugf("Bucket %s: %v", name, value)
	return nil
}

func (s *mockStatter) Incr(name string, tags []string, rate float64) error {
	if s.noop {
		return nil
	}
	log.WithFields(log.WithFn(s.fields)).Debugf("Bucket %s", name)
	return nil
}

func (s *mockStatter) Decr(name string, tags []string, rate float64) error {
	if s.noop {
		return nil
	}
	log.WithFields(log.WithFn(s.fields)).Debugf("Bucket %s", name)
	return nil
}

func (s *mockStatter) Gauge(name string, value float64, tags []string, rate float64) error {
	if s.noop {
		return nil
	}
	log.WithFields(log.WithFn(s.fields)).Debugf("Bucket %s: %v", name, value)
	return nil
}

func (s *mockStatter) Timing(name string, value time.Duration, tags []string, rate float64) error {
	if s.noop {
		return nil
	}
	log.WithFields(log.WithFn(s.fields)).Debugf("Bucket %s: %v", name, value)
	return nil
}

func (s *mockStatter) Histogram(name string, value float64, tags []string, rate float64) error {
	if s.noop {
		return nil
	}
	log.WithFields(log.WithFn(s.fields)).Debugf("Bucket %s: %v", name, value)
	return nil
}

func (s *mockStatter) Unique(bucket string, value string) error {
	if s.noop {
		return nil
	}
	log.WithFields(log.WithFn(s.fields)).Debugf("Bucket %s: %v", bucket, value)
	return nil
}

func (s *mockStatter) Close() error {
	if s.noop {
		return nil
	}
	log.WithFields(log.WithFn(s.fields)).Debugf("closed at %s", time.Now())
	return nil
}
