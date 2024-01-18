// 2023/6/29 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	pq "gitee.com/opengauss/openGauss-connector-go-pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"sync"
	"time"
)

// Servers contains a collection of servers to OpenGauss.
type Servers struct {
	dsn        string
	m          sync.Mutex
	servers    map[string]*Server
	opts       []ServerOpt
	dsnSetting map[string]string
	collStatus map[string]bool

	autoDiscoverOption
	metricMap
}

// NewServers creates a collection of servers to OpenGauss.
func NewServers(dsn string,
	discOption autoDiscoverOption,
	metricMap2 metricMap,
	opts ...ServerOpt) (*Servers, error) {
	dsnSetting, err := pq.ParseURLToMap(dsn)
	if err != nil {
		log.Errorf("Unable to parse DSN (%s): %v", ShadowDSN(dsn), err)
		return nil, err
	}
	servers := &Servers{
		dsn:                dsn,
		servers:            make(map[string]*Server),
		opts:               opts,
		dsnSetting:         dsnSetting,
		collStatus:         map[string]bool{},
		autoDiscoverOption: discOption,
		metricMap:          metricMap2,
	}
	return servers, nil
}

// ScrapeDSN
// -. Connect to the database
// -. Determine the Auto-discover database
//
//	+. Query the database list
//	+. Generate lists based on include-database/exclude-database
//	+. Traverse the database list connection to generate the server
//	+. Clean up old servers
//
// -. Traverse the server collection
func (s *Servers) ScrapeDSN(ch chan<- prometheus.Metric) {
	server, err := s.GetServer(s.dsn)
	if err != nil {
		server.collectorServerInternalMetrics(ch)
		log.Errorf("discoverDatabaseDSNs error opening connection to database (%s): %v", ShadowDSN(s.dsn), err)
		return
	}
	dbMaps, err := server.QueryDatabases()
	if err != nil {
		log.Errorf("QueryDatabases error (%s): %v", ShadowDSN(s.dsn), err)
	}
	// 设置db信息. 根据查询进行关键字段转码
	server.SetDBInfoMap(dbMaps)
	if s.autoDiscovery {
		if len(dbMaps) > 0 {
			s.discoveryServer(dbMaps)
		}
	}
	s.collStatus = map[string]bool{}
	for i := range s.servers {
		server = s.servers[i]
		_, ok := s.collStatus[server.fingerprint]
		// 如果同一个ip+端口采集过一次,说明公共指标已采集,不需要在采集了
		if ok {
			server.notCollInternalMetrics = true
			_ = server.ScrapeWithMetric(ch, s.priMetricMap)
		} else {
			server.notCollInternalMetrics = false
			_ = server.ScrapeWithMetric(ch, s.allMetricMap)
			s.collStatus[server.fingerprint] = true
		}
	}
}

func (s *Servers) discoveryServer(dbMaps map[string]*DBInfo) {
	dsnSetting := make(map[string]string)
	for k, v := range s.dsnSetting {
		dsnSetting[k] = v
	}
	var dsnMap = map[string]bool{
		s.dsn: true,
	}
	newDBNames := s.genDiscoveryDBNames(dbMaps)
	for _, dbName := range newDBNames {
		dsnSetting[DSNDatabase] = dbName
		dsn := genDSNString(dsnSetting)
		server, _ := s.GetServer(dsn)
		// 设置db信息
		server.SetDBInfoMap(dbMaps)
		dsnMap[dsn] = true
	}
	for _, server := range s.servers {
		_, ok := dsnMap[server.dsn]
		if ok {
			continue
		}
		_ = server.Close()
		delete(s.servers, server.dsn)
	}
}

func (s *Servers) genDiscoveryDBNames(dbMaps map[string]*DBInfo) []string {
	var newDBNames []string
	for dbName := range dbMaps {
		if len(s.includeDatabases) > 0 {
			if Contains(s.includeDatabases, dbName) {
				newDBNames = append(newDBNames, dbName)
				continue
			}
		} else if len(s.excludedDatabases) > 0 {
			if Contains(s.excludedDatabases, dbName) {
				continue
			}
			newDBNames = append(newDBNames, dbName)
		} else {
			newDBNames = append(newDBNames, dbName)
		}
	}
	return newDBNames
}

// GetServer returns established connection from a collection.
func (s *Servers) GetServer(dsn string) (*Server, error) {
	s.m.Lock()
	defer s.m.Unlock()
	var err error
	var ok bool
	errCount := 0 // start at zero because we increment before doing work
	retries := 3
	var server *Server
	for {
		if errCount++; errCount > retries {
			return server, err
		}
		server, ok = s.servers[dsn]
		if !ok {
			server, err = NewServer(dsn, s.opts...)
			if err != nil {
				log.Errorf("GetServer NewServer %s err %s", server.fingerprint, err)
				time.Sleep(1 * time.Second)
				continue
			}
			s.servers[dsn] = server
		}
		if !server.UP {
			if err = server.ConnectDatabase(); err != nil {
				log.Errorf("GetServer ConnectDatabase %s err %s", server.fingerprint, err)
				time.Sleep(1 * time.Second)
				continue
			}
		}
		if err = server.Ping(); err != nil {
			// delete(s.servers, dsn)
			log.Errorf("ping %s err %s", server.fingerprint, err)
			time.Sleep(time.Duration(errCount) * time.Second)
			continue
		}
		break
	}

	if err = server.getBaseInfo(); err != nil {
		return server, err
	}

	return server, nil
}

// Close disconnects from all known servers.
func (s *Servers) Close() {
	s.m.Lock()
	defer s.m.Unlock()
	for _, server := range s.servers {
		if err := server.Close(); err != nil {
			log.Errorf("failed to close connection to %q: %v", server, err)
		}
	}
}
