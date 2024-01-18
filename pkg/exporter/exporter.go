// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	"sync"
	"time"
)

type Exporter struct {
	disableCache           bool // always execute query when been scrapped
	failFast               bool // fail fast instead fof waiting during start-up ?
	disableSettingsMetrics bool
	timeToString           bool
	parallel               int
	namespace              string
	configPath             string // config file path /directory
	dsn                    []string
	tags                   []string
	servers                []*Servers
	collStatus             map[string]bool
	constantLabels         prometheus.Labels // 用户定义标签

	autoDiscoverOption
	metricMap

	lock sync.RWMutex // export lock

	scrapeBegin time.Time // server level scrape begin
	scrapeDone  time.Time // server last scrape done
	exportInit  time.Time // server init timestamp

	configFileError  *prometheus.GaugeVec // 读取配置文件失败采集
	exporterUp       prometheus.Gauge     // exporter level: always set ot 1
	exporterUptime   prometheus.Gauge     // exporter level: primary target server uptime (exporter itself)
	lastScrapeTime   prometheus.Gauge     // exporter level: last scrape timestamp
	scrapeDuration   prometheus.Gauge     // exporter level: seconds spend on scrape
	scrapeTotalCount prometheus.Counter   // exporter level: total scrape count of this server
	scrapeErrorCount prometheus.Counter   // exporter level: error scrape count
}

// NewExporter New Exporter
func NewExporter(opts ...Opt) (e *Exporter, err error) {
	e = &Exporter{
		parallel:   1,
		exportInit: time.Now(),
		metricMap: metricMap{
			allMetricMap: defaultMonList, // default metric
			priMetricMap: map[string]*QueryInstance{},
		},
	}
	for _, opt := range opts {
		opt(e)
	}

	e.initDefaultMetric()

	if err := e.loadConfig(); err != nil {
		return nil, err
	}
	e.setupInternalMetrics()
	e.setupServers()

	if e.parallel == 0 {
		e.parallel = 1
	}
	return e, nil
}

// initDefaultMetric init default metric
func (e *Exporter) initDefaultMetric() {
	for _, q := range e.allMetricMap {
		_ = q.Check()
	}
}

// loadConfig Load the configuration file, the same indicator in the configuration file overwrites the default configuration
// 加载配置文件,配置文件里相同指标覆盖默认配置
func (e *Exporter) loadConfig() error {
	if e.configPath == "" {
		return nil
	}
	queryMap, err := LoadConfig(e.configPath)
	if err != nil {
		return err
	}
	for name, query := range queryMap {
		var found, found1 bool
		for defName, defQuery := range e.allMetricMap {
			if strings.EqualFold(defQuery.Name, query.Name) {
				e.allMetricMap[defName] = query
				found = true
				break
			}
		}
		if !found {
			e.allMetricMap[name] = query
		}
		// 如果是通用指标不判断私有
		if query.Public {
			continue
		}
		for defName, defQuery := range e.priMetricMap {
			if strings.EqualFold(defQuery.Name, query.Name) {
				e.priMetricMap[defName] = query
				found1 = true
				break
			}
		}
		if !found1 {
			e.priMetricMap[name] = query
		}
	}
	return nil
}

func (e *Exporter) setupServers() {
	for i := range e.dsn {
		dsn := e.dsn[i]
		s, err := NewServers(dsn,
			e.autoDiscoverOption,
			e.metricMap,
			ServerWithLabels(e.constantLabels),
			ServerWithNamespace(e.namespace),
			ServerWithDisableSettingsMetrics(e.disableSettingsMetrics),
			ServerWithDisableCache(e.disableCache),
			ServerWithTimeToString(e.timeToString),
			ServerWithParallel(e.parallel),
		)
		if err != nil {
			continue
		}
		e.servers = append(e.servers, s)
	}
}

// Describe implement prometheus.Collector
// -> Collect
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})

	go func() {
		for m := range metricCh {
			ch <- m.Desc()
		}
		close(doneCh)
	}()

	e.Collect(metricCh)
	close(metricCh)
	<-doneCh
}

// Collect
//
//	Collect ->
//		scrape
//			go servers.ScrapeDSN
//				GetServer
//				autoDiscovery
//				for server collect
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.scrape(ch)
	e.collectServerMetrics()
	e.collectInternalMetrics(ch)
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.lock.Lock()
	defer e.lock.Unlock()
	// 设置采集开始时间
	e.scrapeBegin = time.Now()
	wg := sync.WaitGroup{}
	// 根据dsn并发采集.
	for i := range e.servers {
		wg.Add(1)
		go func(servers *Servers) {
			defer wg.Done()
			servers.ScrapeDSN(ch)
		}(e.servers[i])
	}
	wg.Wait()
	// 设置结束开始时间
	e.scrapeDone = time.Now()
	// 最后采集时间
	e.lastScrapeTime.Set(float64(e.scrapeDone.Unix()))
	// 采集耗时
	e.scrapeDuration.Set(e.scrapeDone.Sub(e.scrapeBegin).Seconds())
	// 在线时间
	e.exporterUptime.Set(time.Now().Sub(e.exportInit).Seconds())
	// 在线
	e.exporterUp.Set(1)
}

func (e *Exporter) collectServerMetrics() {
	for _, server := range e.servers {
		for _, s := range server.servers {
			e.scrapeTotalCount.Add(float64(s.ScrapeTotalCount))
			e.scrapeErrorCount.Add(float64(s.ScrapeErrorCount))
		}
	}
}

func (e *Exporter) collectInternalMetrics(ch chan<- prometheus.Metric) {
	ch <- e.exporterUp
	ch <- e.exporterUptime
	ch <- e.lastScrapeTime
	ch <- e.scrapeTotalCount
	ch <- e.scrapeErrorCount
	ch <- e.scrapeDuration
}

func (e *Exporter) Close() {
	for _, s := range e.servers {
		s.Close()
	}
}
