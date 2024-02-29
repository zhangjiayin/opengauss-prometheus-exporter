// Copyright © 2021 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *Server) execSQL(ctx context.Context, sqlText string) (*sql.Rows, error) {
	ch := make(chan struct{}, 0)
	var (
		rows *sql.Rows
		err  error
	)
	go func() {
		rows, err = s.db.Query(sqlText)
		ch <- struct{}{}
	}()
	select {
	case <-ch:
		return rows, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Server) doCollectMetric(queryInstance *QueryInstance) ([]prometheus.Metric, []error, error) {
	// 根据版本获取查询sql
	query := queryInstance.GetQuerySQL(s.lastMapVersion, s.primary)
	if query == nil {
		// Return success (no pertinent data)
		return []prometheus.Metric{}, []error{}, nil
	}

	// Don't fail on a bad scrape of one metric
	var (
		rows       *sql.Rows
		err        error
		ctx        = context.Background()
		metricName = queryInstance.Name
	)
	begin := time.Now()
	// TODO disable timeout
	if query.Timeout > 0 { // if timeout is provided, use context
		var cancel context.CancelFunc
		log.Debugf("Collect Metric [%s] query with time limit: %v", query.Name, query.TimeoutDuration())
		ctx, cancel = context.WithTimeout(context.Background(), query.TimeoutDuration())
		defer cancel()
	}
	log.Debugf("Collect Metric [%s] query sql %s", queryInstance.Name, query.SQL)
	rows, err = s.execSQL(ctx, query.SQL)
	end := time.Now().Sub(begin).Milliseconds()

	log.Debugf("Collect Metric [%s] query using time %vms", queryInstance.Name, end)
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			log.Errorf("Collect Metric [%s] query timeout %v", queryInstance.Name, query.TimeoutDuration())
			err = fmt.Errorf("timeout %v %s", query.TimeoutDuration(), err)
		} else {
			log.Errorf("Collect Metric [%s] query err %s", queryInstance.Name, err)
		}
		return []prometheus.Metric{}, []error{},
			fmt.Errorf("Collect Metric [%s] query on database %q err %s ", metricName, s, err)
	}
	defer rows.Close()
	var columnNames []string
	columnNames, err = rows.Columns()
	if err != nil {
		log.Errorf("Collect Metric [%s] fetch Columns err %s", queryInstance.Name, err)
		return []prometheus.Metric{}, []error{}, errors.New(fmt.Sprintln("Error retrieving column list for: ", metricName, err))
	}

	// Make a lookup map for the column indices
	var columnIdx = make(map[string]int, len(columnNames))
	for i, n := range columnNames {
		columnIdx[n] = i
	}
	nonfatalErrors := []error{}
	var list [][]interface{}
	for rows.Next() {
		var columnData = make([]interface{}, len(columnNames))
		var scanArgs = make([]interface{}, len(columnNames))
		for i := range columnData {
			scanArgs[i] = &columnData[i]
		}
		err = rows.Scan(scanArgs...)
		if err != nil {
			log.Errorf("Collect Metric [%s] fetch rows.Scan err %s", queryInstance.Name, err)
			nonfatalErrors = append(nonfatalErrors, err)
			break
		}
		list = append(list, columnData)
	}
	if err = rows.Err(); err != nil {
		log.Debugf("Collect Metric [%s] fetch data rows.Err() %s", metricName, err)
		nonfatalErrors = append(nonfatalErrors, err)
	}
	end = time.Now().Sub(begin).Milliseconds()
	log.Debugf("Collect Metric [%s] fetch total time %vms", queryInstance.Name, end)

	metrics := make([]prometheus.Metric, 0)
	for i := range list {
		metric, errs := s.procRows(queryInstance, columnNames, columnIdx, list[i])
		if len(errs) > 0 {
			nonfatalErrors = append(nonfatalErrors, errs...)
		}
		if metric != nil {
			metrics = append(metrics, metric...)
		}
	}
	return metrics, nonfatalErrors, nil
}

func (s *Server) decode(queryInstance *QueryInstance, data interface{}, label, dbName string) (string, error) {
	v, _ := dbToString(data, s.timeToString)
	col := queryInstance.GetColumn(label, s.labels)
	if col == nil {
		return v, nil
	}
	if !col.CheckUTF8 {
		return v, nil
	}
	if utf8.ValidString(v) {
		return v, nil
	}
	// 检查编码是否UTF8,不是则改为空
	if s.dbInfoMap == nil {
		return "", nil
	}
	if dbName == "" {
		return "", nil
	}
	dbInfo, ok := s.dbInfoMap[dbName]
	if !ok {
		return "", nil
	}
	if dbInfo == nil {
		return "", nil
	}
	if dbInfo.Charset == "" {
		return "", nil
	}
	if s.clientEncoding == UTF8 && dbInfo.Charset == UTF8 {
		return "", nil
	}
	b, err := DecodeByte([]byte(v), dbInfo.Charset)
	if err != nil {
		log.Errorf("DecodeByte %s", err)
		return "", nil
	}
	return string(b), nil
}

func (s *Server) procRows(queryInstance *QueryInstance, columnNames []string, columnIdx map[string]int, columnData []interface{}) ([]prometheus.Metric, []error) {
	// Get the label values for this row.
	metrics := make([]prometheus.Metric, 0)
	nonfatalErrors := []error{}
	labels := make([]string, len(queryInstance.LabelNames))
	var dbName string
	dbNameLabel := queryInstance.dbNameLabel
	if dbNameLabel != "" {
		dbName, _ = dbToString(columnData[columnIdx[dbNameLabel]], s.timeToString)
	}
	for idx, label := range queryInstance.LabelNames {
		v, err := s.decode(queryInstance, columnData[columnIdx[label]], label, dbName)
		if err != nil {
			log.Errorf("decode %s", err)
		}
		labels[idx] = v
	}
	// Loop over column names, and match to scan data. Unknown columns
	// will be filled with an untyped metric number *if* they can be
	// converted to float64s. NULLs are allowed and treated as NaN.
	for idx, columnName := range columnNames {
		col := queryInstance.GetColumn(columnName, s.labels)
		metric, err := s.newMetric(queryInstance, col, columnName, columnData[idx], labels)
		if err != nil {
			log.Errorf("newMetric %s", err)
			nonfatalErrors = append(nonfatalErrors, err)
			continue
		}
		if metric != nil {
			metrics = append(metrics, metric)
		}
	}
	return metrics, nonfatalErrors
}

func (s *Server) newMetric(queryInstance *QueryInstance, col *Column, columnName string, colValue interface{},
	labels []string) (metric prometheus.Metric, err error) {
	var (
		desc       *prometheus.Desc
		value      float64
		valueOK    bool
		metricName = queryInstance.Name
		valueType  prometheus.ValueType
	)
	if col == nil {
		return nil, nil
	}
	if col.DisCard {
		return nil, nil
	}
	if col.Histogram {
		return nil, nil
	}
	if strings.EqualFold(col.Usage, MappedMETRIC) {
		return nil, nil
	}
	desc = col.PrometheusDesc
	valueType = col.PrometheusType
	value, valueOK = dbToFloat64(colValue)
	if !valueOK {
		return nil, errors.New(fmt.Sprintln("Unexpected error parsing column: ", metricName, columnName, colValue))
	}
	defer RecoverErr(&err)
	metric = prometheus.MustNewConstMetric(desc, valueType, value, labels...)
	return metric, nil
}
