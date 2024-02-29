// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/blang/semver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

var (
	serverLabelName = "server"
	// staticLabelName = "static"
)

// ServerOpt configures a server.
type ServerOpt func(*Server)

// ServerWithLabels configures a set of labels.
func ServerWithLabels(labels prometheus.Labels) ServerOpt {
	return func(s *Server) {
		for k, v := range labels {
			s.labels[k] = v
		}
	}
}

// ServerWithNamespace will specify metric namespace, by default is pg or pgbouncer
func ServerWithNamespace(namespace string) ServerOpt {
	return func(s *Server) {
		s.namespace = namespace
	}
}

// ServerWithDisableSettingsMetrics will specify metric namespace, by default is pg or pgbouncer
func ServerWithDisableSettingsMetrics(b bool) ServerOpt {
	return func(s *Server) {
		s.disableSettingsMetrics = b
	}
}

// ServerWithDisableCache  will specify metric namespace, by default is pg or pgbouncer
func ServerWithDisableCache(b bool) ServerOpt {
	return func(s *Server) {
		s.disableCache = b
	}
}
func ServerWithTimeToString(b bool) ServerOpt {
	return func(s *Server) {
		s.timeToString = b
	}
}

func ServerWithParallel(i int) ServerOpt {
	return func(s *Server) {
		s.parallel = i
	}
}

type Server struct {
	fingerprint            string
	dsn                    string
	db                     *sql.DB
	labels                 prometheus.Labels
	primary                bool
	namespace              string // default prometheus namespace from cmd args
	disableSettingsMetrics bool
	notCollInternalMetrics bool // 不采集部分指标
	disableCache           bool
	timeToString           bool

	parallel int
	// Last version used to calculate metric map. If mismatch on scrape,
	// then maps are recalculated.
	lastMapVersion semver.Version
	lock           sync.RWMutex
	// Currently cached metrics
	cacheMtx         sync.Mutex
	metricCache      map[string]*cachedMetrics
	UP               bool
	ScrapeTotalCount int64     // 采集指标个数
	ScrapeErrorCount int64     // 采集失败个数
	scrapeBegin      time.Time // server level scrape begin
	scrapeDone       time.Time // server last scrape done

	up               prometheus.Gauge
	recovery         prometheus.Gauge   // postgres is in recovery ?
	lastScrapeTime   prometheus.Gauge   // exporter level: last scrape timestamp
	scrapeDuration   prometheus.Gauge   // exporter level: seconds spend on scrape
	scrapeTotalCount prometheus.Counter // exporter level: total scrape count of this server
	scrapeErrorCount prometheus.Counter // exporter level: error scrape count

	queryCacheTTL          map[string]float64 // internal query metrics: cache time to live
	queryScrapeTotalCount  map[string]float64 // internal query metrics: total executed
	queryScrapeHitCount    map[string]float64 // internal query metrics: times serving from hit cache
	queryScrapeErrorCount  map[string]float64 // internal query metrics: times failed
	queryScrapeMetricCount map[string]float64 // internal query metrics: number of metrics scrapped
	queryScrapeDuration    map[string]float64 // internal query metrics: time spend on executing
	clientEncoding         string
	dbInfoMap              map[string]*DBInfo
	dbName                 string
}

type DBInfo struct {
	DBName           string
	Charset          string
	Datcompatibility string
}

// Close disconnects from OpenGauss.
func (s *Server) Close() error {
	if s.db == nil {
		return nil
	}
	s.UP = false

	return s.db.Close()
}

// Ping checks connection availability and possibly invalidates the connection if it fails.
func (s *Server) Ping() error {
	if err := s.db.Ping(); err != nil {
		if closeErr := s.Close(); closeErr != nil {
			log.Errorf("Error while closing non-pinging DB connection to %q: %v", s, closeErr)
		}
		return err
	}
	return nil
}

// String returns server's fingerprint.
func (s *Server) String() string {
	return s.labels[serverLabelName]
}

func (s *Server) setupServerInternalMetrics() error {
	s.scrapeTotalCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: s.namespace, ConstLabels: s.labels,
		Subsystem: "exporter_query", Name: "scrape_total_count", Help: "times exporter was scraped for metrics",
	})
	s.scrapeErrorCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: s.namespace, ConstLabels: s.labels,
		Subsystem: "exporter_query", Name: "scrape_error_count", Help: "times exporter was scraped for metrics and failed",
	})
	s.scrapeDuration = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: s.namespace, ConstLabels: s.labels,
		Subsystem: "exporter_query", Name: "scrape_duration", Help: "seconds exporter spending on scrapping",
	})
	s.lastScrapeTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: s.namespace, ConstLabels: s.labels,
		Subsystem: "exporter_query", Name: "last_scrape_time", Help: "seconds exporter spending on scrapping",
	})
	s.recovery = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: s.namespace, ConstLabels: s.labels,
		Name: "in_recovery", Help: "server is in recovery mode? 1 for yes 0 for no",
	})
	s.up = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: s.namespace, ConstLabels: s.labels,
		Name: "up", Help: "always be 1 if your could retrieve metrics",
	})
	return nil
}

func (s *Server) collectorServerInternalMetrics(ch chan<- prometheus.Metric) {
	if s.notCollInternalMetrics {
		return
	}
	s.lock.RLock()
	defer s.lock.RUnlock()

	_ = s.setupServerInternalMetrics()
	if s.UP {
		s.up.Set(1)
		if s.primary {
			s.recovery.Set(0)
		} else {
			s.recovery.Set(1)
		}
	} else {
		s.up.Set(0)
		s.scrapeErrorCount.Add(1)
	}
	if s.scrapeBegin.IsZero() {
		s.scrapeBegin = time.Now()
	}
	s.scrapeDone = time.Now()
	// 最后采集时间
	s.lastScrapeTime.Set(float64(s.scrapeDone.Unix()))
	// 采集耗时
	s.scrapeDuration.Set(s.scrapeDone.Sub(s.scrapeBegin).Seconds())

	versionDesc := prometheus.NewDesc(fmt.Sprintf("%s_%s", s.namespace, "version"),
		"Version string as reported by OpenGauss", []string{"version", "short_version"}, s.labels)
	version := prometheus.MustNewConstMetric(versionDesc,
		prometheus.UntypedValue, 1, s.lastMapVersion.String(), s.lastMapVersion.String())
	s.scrapeTotalCount.Add(float64(s.ScrapeTotalCount))
	s.scrapeErrorCount.Add(float64(s.ScrapeErrorCount))

	ch <- s.up
	ch <- s.recovery
	ch <- s.scrapeTotalCount
	ch <- s.scrapeErrorCount
	ch <- s.scrapeDuration
	ch <- s.lastScrapeTime
	ch <- version

}

func (s *Server) CheckConn() error {
	if s.db == nil || !s.UP {
		return fmt.Errorf("not connect database")
	}
	return nil
}

func (s *Server) DBRole() string {
	if s.primary {
		return "primary"
	}
	return "standby"
}

func (s *Server) SetDBInfoMap(info map[string]*DBInfo) {
	s.dbInfoMap = info
}

// QueryDatabases 连接数据查询监控指标
func (s *Server) QueryDatabases() (map[string]*DBInfo, error) {
	rows, err := s.db.Query(`SELECT d.datname,pg_encoding_to_char(d.encoding) as og_charset, d.datcompatibility FROM pg_database d
	WHERE d.datallowconn = true AND d.datistemplate = false`) // nolint: safesql
	if err != nil {
		return nil, fmt.Errorf("Error retrieving databases: %v", err)
	}
	defer rows.Close() // nolint: errcheck

	result := map[string]*DBInfo{}
	for rows.Next() {
		var (
			databaseName, charset, datcompatibility string
		)
		err = rows.Scan(&databaseName, &charset, &datcompatibility)
		if err != nil {
			return nil, errors.New(fmt.Sprintln("Error retrieving rows:", err))
		}
		// result = append(result, databaseName)
		result[databaseName] = &DBInfo{
			DBName:           databaseName,
			Charset:          charset,
			Datcompatibility: datcompatibility,
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// getBaseInfo 查询数据库基本信息
// 1. 版本
// 2. 客户端编码
// 3. 恢复模式
func (s *Server) getBaseInfo() error {
	if err := s.CheckConn(); err != nil {
		return err
	}
	var (
		versionString, clientEncoding, currentDatabase string
		b                                              bool
	)
	sqlText := "SELECT version(),current_setting('client_encoding'),pg_is_in_recovery(),current_database()"
	logrus.Debugf(sqlText)
	err := s.db.QueryRow(sqlText).Scan(&versionString, &clientEncoding, &b, &currentDatabase)
	if err != nil {
		return err
	}
	s.primary = !b
	s.clientEncoding = clientEncoding
	semanticVersion, err := parseVersionSem(versionString)
	if err != nil {
		return fmt.Errorf("Error parsing version string err %s ", err)
	}
	s.lastMapVersion = semanticVersion
	s.dbName = currentDatabase
	return nil
}

func (s *Server) ConnectDatabase() error {
	if s.db != nil {
		if err := s.Ping(); err == nil {
			s.UP = true
			return nil
		}
		s.db.Close()
	}
	db, err := sql.Open("opengauss", s.dsn)
	if err != nil {
		s.UP = false
		return err
	}
	s.db = db
	if err = s.Ping(); err != nil {
		s.UP = false
		return err
	}
	s.db.SetConnMaxIdleTime(120 * time.Second)
	s.db.SetMaxIdleConns(s.parallel)
	// s.db.SetMaxOpenConns(s.parallel)
	s.UP = true
	return nil
}

func NewServer(dsn string, opts ...ServerOpt) (*Server, error) {
	// 获取server名称 ip:port
	fingerprint, err := parseFingerprint(dsn)
	if err != nil {
		return nil, err
	}

	log.Infof("Established new database connection to %q.", fingerprint)

	s := &Server{
		fingerprint: fingerprint,
		dsn:         dsn,
		primary:     false,
		labels: prometheus.Labels{
			serverLabelName: fingerprint,
		},
		metricCache: make(map[string]*cachedMetrics),
	}

	for _, opt := range opts {
		opt(s)
	}

	if err = s.ConnectDatabase(); err != nil {
		return s, err
	}
	return s, nil
}
