// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_Exporter(t *testing.T) {
	exporter, err := NewExporter(
		WithParallel(2),
		WithConfig("../../og_exporter_default.yaml"),
	)
	if err != nil {
		t.Error(err)
		return
	}
	t.Run("initDefaultMetric", func(t *testing.T) {
		exporter.initDefaultMetric()
	})
	t.Run("LoadConfig", func(t *testing.T) {
		exporter.configPath = "a1.yaml"
		err := exporter.loadConfig()
		assert.Error(t, err)
	})
	t.Run("GetMetricsList", func(t *testing.T) {
		list := exporter.GetMetricsList()
		assert.NotNil(t, list)
	})
	t.Run("LoadConfig_configPath_null", func(t *testing.T) {
		exporter.configPath = ""
		err := exporter.loadConfig()
		assert.NoError(t, err)
	})
	t.Run("Describe", func(t *testing.T) {
		ch := make(chan *prometheus.Desc, 100)
		exporter.Describe(ch)
		close(ch)
	})
	t.Run("Collect", func(t *testing.T) {
		ch := make(chan prometheus.Metric, 100)
		exporter.Collect(ch)
		close(ch)
	})
	// t.Run("Close", func(t *testing.T) {
	// 	exporter.Check()
	// })
	t.Run("Close", func(t *testing.T) {
		exporter.Close()
	})
}

func TestExporter_genDiscDsn(t *testing.T) {
	type fields struct {
		excludedDatabases []string
		includeDatabases  []string
	}
	type args struct {
		dbNames   map[string]*DBInfo
		parsedDSN map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []string
	}{
		{
			name: "none",
			args: args{
				dbNames: map[string]*DBInfo{
					"a1": {
						DBName:           "a1",
						Charset:          "",
						Datcompatibility: "",
					},
					"a2": {
						DBName:           "a2",
						Charset:          "",
						Datcompatibility: "",
					},
				},
				parsedDSN: map[string]string{},
			},
			want: []string{"a1", "a2"},
		},
		{
			name: "include",
			args: args{
				dbNames: map[string]*DBInfo{
					"a1": {
						DBName:           "a1",
						Charset:          "",
						Datcompatibility: "",
					},
					"a2": {
						DBName:           "a2",
						Charset:          "",
						Datcompatibility: "",
					},
					"a3": {
						DBName:           "a3",
						Charset:          "",
						Datcompatibility: "",
					},
					"a4": {
						DBName:           "a4",
						Charset:          "",
						Datcompatibility: "",
					},
				},
				parsedDSN: map[string]string{},
			},
			fields: fields{
				includeDatabases:  []string{"a1", "a2"},
				excludedDatabases: []string{"a1", "a3", "a4"},
			},
			want: []string{"a1", "a2"},
		},
		{
			name: "exclude",
			args: args{
				dbNames: map[string]*DBInfo{
					"a1": {
						DBName:           "a1",
						Charset:          "",
						Datcompatibility: "",
					},
					"a2": {
						DBName:           "a2",
						Charset:          "",
						Datcompatibility: "",
					},
					"a3": {
						DBName:           "a3",
						Charset:          "",
						Datcompatibility: "",
					},
					"a4": {
						DBName:           "a4",
						Charset:          "",
						Datcompatibility: "",
					},
				},
				parsedDSN: map[string]string{},
			},
			fields: fields{
				excludedDatabases: []string{"a3", "a4"},
			},
			want: []string{"a1", "a2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Servers{
				autoDiscoverOption: autoDiscoverOption{
					excludedDatabases: tt.fields.excludedDatabases,
					includeDatabases:  tt.fields.includeDatabases,
				},
			}
			assert.Equalf(t, tt.want, s.genDiscoveryDBNames(tt.args.dbNames), "genDiscDsn(%v, %v)", tt.args.dbNames, tt.args.parsedDSN)
		})
	}
}
