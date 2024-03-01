// Copyright © 2021 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"strings"
	"sync"
	"time"
)

type metricError struct {
	lock   sync.Mutex
	Errors map[string]error
	Count  int64
}

func (e *metricError) addError(metricName string, err error) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.Errors[metricName] = err
	e.Count++
}

// ScrapeWithMetric loads metrics.
func (s *Server) ScrapeWithMetric(ch chan<- prometheus.Metric, queryMetric map[string]*QueryInstance) error {
	if err := s.CheckConn(); err != nil {
		return err
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	defer func() {
		s.collectorServerInternalMetrics(ch)
	}()
	s.scrapeBegin = time.Now()
	var err error
	if !s.disableSettingsMetrics && !s.notCollInternalMetrics {
		if err = s.querySettings(ch); err != nil {
			err = fmt.Errorf("error retrieving settings: %s", err)
		}
	}
	errMap := s.queryMetrics(ch, queryMetric)
	if len(errMap) > 0 {
		err = fmt.Errorf("queryMetrics returned %d errors", len(errMap))
	}
	return err
}

// 查询监控指标. 先判断是否读取缓存. 禁用缓存或者缓存超时,则读取数据库
// 启动 parallel 个协程,每个协程固定一个conn，监听指标通道
func (s *Server) queryMetrics(ch chan<- prometheus.Metric, queryMetric map[string]*QueryInstance) map[string]error {

	var (
		parallel     = s.parallel
		metricChan   = make(chan *QueryInstance, parallel)
		wg           = sync.WaitGroup{}
		metricErrors = &metricError{
			Errors: map[string]error{},
			Count:  0,
		}
	)
	go func() {
		for _, metric := range queryMetric {
			metricChan <- metric
		}
		close(metricChan)
	}()
	wg.Add(parallel)
	for i := 0; i < parallel; i++ {
		go func(workNum int) {
			defer wg.Done()
			conn, err := s.db.Conn(context.Background())
			if err != nil {
				return
			}
			defer conn.Close()
			s.startQueryMetricThread(conn, ch, metricChan, metricErrors)
		}(i)
	}
	wg.Wait()
	s.ScrapeErrorCount = metricErrors.Count
	return metricErrors.Errors
}

func (s *Server) startQueryMetricThread(conn *sql.Conn, ch chan<- prometheus.Metric, metricChan chan *QueryInstance, metricErrors *metricError) error {
	for {
		select {
		case metric, ok := <-metricChan:
			if !ok {
				return nil
			}
			err := s.queryMetric(ch, metric, conn)
			if err != nil {
				// 存在并发写入问题. 改成结构体加锁
				metricErrors.addError(metric.Name, err)
			}
		}
	}
}

func (s *Server) queryMetric(ch chan<- prometheus.Metric, queryInstance *QueryInstance, conn *sql.Conn) error {
	var (
		metricName     = queryInstance.Name
		scrapeMetric   = false // Whether to collect indicators from the database 是否从数据库里采集指标
		cachedMetric   = &cachedMetrics{}
		metrics        []prometheus.Metric
		nonFatalErrors []error
		err            error
	)

	querySQL := queryInstance.GetQuerySQL(s.lastMapVersion, s.primary)
	if querySQL == nil {
		log.Warnf("Collect Metric %s not define querySQL for version %s on %s database ", metricName, s.lastMapVersion.String(), s.DBRole())
		return nil
	}
	if strings.EqualFold(querySQL.Status, statusDisable) {
		log.Debugf("Collect Metric %s disable. skip", metricName)
		return nil
	}

	// 记录采集总个数
	s.ScrapeTotalCount++

	// Determine whether to enable caching and cache expiration 判断是否启用缓存和缓存过期
	if !s.disableCache {
		var found bool
		// Check if the metric is cached
		s.cacheMtx.Lock()
		cachedMetric, found = s.metricCache[metricName]
		s.cacheMtx.Unlock()
		// If found, check if needs refresh from cache
		if !found {
			scrapeMetric = true
		} else if !cachedMetric.IsValid(querySQL.TTL) {
			scrapeMetric = true
		}
		if cachedMetric != nil && (len(cachedMetric.nonFatalErrors) > 0 || len(cachedMetric.metrics) == 0) {
			scrapeMetric = true
		}
	} else {
		scrapeMetric = true
	}
	if scrapeMetric {
		metrics, nonFatalErrors, err = s.doCollectMetric(queryInstance, conn)
	} else {
		log.Debugf("Collect Metric [%s] on %s use cache", metricName, s.dbName)
		metrics, nonFatalErrors = cachedMetric.metrics, cachedMetric.nonFatalErrors
	}

	// Serious error - a namespace disappeared
	if err != nil {
		nonFatalErrors = append(nonFatalErrors, err)
		log.Errorf("Collect Metric [%s] on %s err %s", metricName, s.dbName, err)
	}
	// Non-serious errors - likely version or parsing problems.
	if len(nonFatalErrors) > 0 {
		var errText string
		for _, err := range nonFatalErrors {
			log.Errorf("Collect Metric [%s] %s nonFatalErrors err %s", metricName, s.dbName, err)
			errText += err.Error()
		}
		err = errors.New(errText)
	}

	// Emit the metrics into the channel
	for _, m := range metrics {
		ch <- m
	}

	if scrapeMetric && queryInstance.TTL > 0 {
		// Only cache if metric is meaningfully cacheable
		s.cacheMtx.Lock()
		s.metricCache[metricName] = &cachedMetrics{
			metrics:        metrics,
			lastScrape:     time.Now(), // 改为查询完时间
			nonFatalErrors: nonFatalErrors,
		}
		s.cacheMtx.Unlock()
	}
	return err
}
