// Copyright © 2021 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	pq "gitee.com/opengauss/openGauss-connector-go-pq"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_genDSNString(t *testing.T) {
	type args struct {
		connStringSettings map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "a1",
			args: args{
				connStringSettings: map[string]string{
					"host":     "localhost",
					"password": "passwordDsn",
					"port":     "55432",
					"sslmode":  "disabled",
					"user":     "userDsn",
				},
			},
			want: "host=localhost password=passwordDsn port=55432 sslmode=disabled user=userDsn",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := genDSNString(tt.args.connStringSettings); got != tt.want {
				t.Errorf("genDSNString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseDSNSettings(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "localhost:55432",
			args: args{
				s: "postgres://userDsn:passwordDsn@localhost:55432/?sslmode=disabled",
			},
			want: map[string]string{
				"host":     "localhost",
				"password": "passwordDsn",
				"port":     "55432",
				"sslmode":  "disabled",
				"user":     "userDsn",
			},
			wantErr: false,
		},
		{
			name: "err",
			args: args{
				s: "user",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "127.0.0.1:5432",
			args: args{
				s: "user=xxx password=xxx host=127.0.0.1 port=5432 dbname=postgres sslmode=disable",
			},
			want: map[string]string{
				"database": "postgres",
				"host":     "127.0.0.1",
				"password": "xxx",
				"port":     "5432",
				"sslmode":  "disable",
				"user":     "xxx",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pq.ParseURLToMap(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDSNSettings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_parseDsn(t *testing.T) {
	type args struct {
		dsn string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "localhost:55432",
			args: args{
				dsn: "postgres://userDsn:passwordDsn@localhost:55432/?sslmode=disabled",
			},
			want: map[string]string{
				"host":     "localhost",
				"password": "passwordDsn",
				"port":     "55432",
				"sslmode":  "disabled",
				"user":     "userDsn",
			},
		},
		{
			name: "localhost:55432",
			args: args{
				dsn: "postgres://userDsn:passwordDsn%3D@localhost:55432/?sslmode=disabled",
			},
			want: map[string]string{
				"host":     "localhost",
				"password": "passwordDsn=",
				"port":     "55432",
				"sslmode":  "disabled",
				"user":     "userDsn",
			},
		},
		{
			name: "127.0.0.1:5432",
			args: args{
				dsn: "user=xxx password=xxx host=127.0.0.1 port=5432 dbname=postgres sslmode=disable",
			},
			want: map[string]string{
				"database": "postgres",
				"host":     "127.0.0.1",
				"password": "xxx",
				"port":     "5432",
				"sslmode":  "disable",
				"user":     "xxx",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pq.ParseURLToMap(tt.args.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDsn() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_parseURLSettings(t *testing.T) {
	type args struct {
		connString string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "localhost:55432",
			args: args{
				connString: "postgres://userDsn:passwordDsn@localhost:55432/?sslmode=disabled",
			},
			want: map[string]string{
				"host":     "localhost",
				"password": "passwordDsn",
				"port":     "55432",
				"sslmode":  "disabled",
				"user":     "userDsn",
			},
			wantErr: false,
		},
		{
			name: "127.0.0.1:5432",
			args: args{
				connString: "user=xxx password=xxx host=127.0.0.1 port=5432 dbname=postgres sslmode=disable",
			},
			want: map[string]string{
				"database": "postgres",
				"host":     "127.0.0.1",
				"password": "xxx",
				"port":     "5432",
				"sslmode":  "disable",
				"user":     "xxx",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pq.ParseURLToMap(tt.args.connString)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseURLSettings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_parseFingerprint(t *testing.T) {
	type args struct {
		url string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "localhost:55432",
			args: args{
				url: "postgres://userDsn:passwordDsn@localhost:55432/?sslmode=disable",
			},
			want: "localhost:55432",
		},
		{
			name: "localhost:55432",
			args: args{
				url: "postgres://userDsn:passwordDsn%3D@localhost:55432/?sslmode=disable",
			},
			want: "localhost:55432",
		},
		{
			name: "127.0.0.1:5432",
			args: args{
				url: "user=xxx password=xxx host=127.0.0.1 port=5432 dbname=postgres sslmode=disable",
			},
			want: "127.0.0.1:5432",
		},
		{
			name: "localhost:1234",
			args: args{
				url: "port=1234",
			},

			want: "localhost:1234",
		},
		{
			name: "example:5432",
			args: args{
				url: "host=example",
			},
			want: "example:5432",
		},
		{
			name: "xyz",
			args: args{
				url: "xyz",
			},
			wantErr: true,
		},
		{
			name: "postgres://gaussdb:secret@localhost:5432/mydb?sslmode=disable&host=/tmp",
			args: args{
				url: "postgres://gaussdb:secret@localhost:5432/mydb?sslmode=disable&host=/tmp",
			},
			want:    "localhost:5432",
			wantErr: false,
		},
		{
			name: "postgres://gaussdb:secret@localhost:5432/mydb?sslmode=disable&host=/tmp",
			args: args{
				url: "postgres://gaussdb:secret@localhost:5432/mydb?sslmode=disable&host=/tmp",
			},
			want:    "localhost:5432",
			wantErr: false,
		},
		{
			name: "postgres://gaussdb:secret@localhost:5432,localhost:5433/mydb?sslmode=disable&host=/tmp",
			args: args{
				url: "postgres://gaussdb:secret@localhost:5432,localhost:5433/mydb?sslmode=disable&host=/tmp",
			},
			want:    "localhost:5432",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFingerprint(tt.args.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFingerprint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
