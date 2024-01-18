// Copyright © 2021 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"fmt"
	"gitee.com/opengauss/openGauss-connector-go-pq"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	DSNHost        = "host"
	DSNLocalhost   = "localhost"
	DSNPort        = "port"
	DSNDefaultPort = "5432"
	DSNDatabase    = "database"
	DSNDBName      = "dbname"
	DSNUser        = "user"
	DSNPassword    = "password"
)

func genDSNString(connStringSettings map[string]string) string {
	var kvs []string
	for k, v := range connStringSettings {
		kvs = append(kvs, fmt.Sprintf("%s=%v", k, v))
	}
	sort.Strings(kvs) // Makes testing easier (not a performance concern)
	return strings.Join(kvs, " ")
}

// ShadowDSN will hide password part of dsn
func ShadowDSN(dsn string) string {
	pDSN, err := url.Parse(dsn)
	if err != nil {
		return ""
	}
	// Blank user info if not nil
	if pDSN.User != nil {
		pDSN.User = url.UserPassword(pDSN.User.Username(), "******")
	}
	return pDSN.String()
}

func parseFingerprint(url string) (string, error) {
	config, err := pq.ParseConfig(url)
	if err != nil {
		return "", err
	}
	var (
		fingerprint         string
		fingerprintHostName string
		fingerprintPort     string
	)
	fingerprintHostName = config.Host
	fingerprintPort = strconv.Itoa(int(config.Port))
	if strings.HasPrefix(fingerprintHostName, "/") {
		fingerprintHostName = DSNLocalhost
	}
	if fingerprintPort == "" {
		fingerprintPort = DSNDefaultPort
	}
	fingerprint = fmt.Sprintf("%s:%s", fingerprintHostName, fingerprintPort)
	return fingerprint, nil
}
