// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/blang/semver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func Test_parseConstLabels(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want prometheus.Labels
	}{
		{
			name: "a=b",
			args: args{s: "a=b"},
			want: prometheus.Labels{
				"a": "b",
			},
		},
		{
			name: "null",
			args: args{s: ""},
			want: nil,
		},
		{
			name: "a=b, c=d",
			args: args{s: "a=b, c=d"},
			want: prometheus.Labels{
				"a": "b",
				"c": "d",
			},
		},
		{
			name: "a=b, xyz",
			args: args{s: "a=b, xyz"},
			want: prometheus.Labels{
				"a": "b",
			},
		},
		{
			name: "a=",
			args: args{s: "a="},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseConstLabels(tt.args.s)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShadowDSN(t *testing.T) {
	type args struct {
		dsn string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "localhost:55432",
			args: args{
				dsn: "postgres://userDsn:passwordDsn@localhost:55432/?sslmode=disabled",
			},
			want: "postgres://userDsn:%2A%2A%2A%2A%2A%2A@localhost:55432/?sslmode=disabled",
		},
		{
			name: "localhost:55432",
			args: args{
				dsn: "postgres://gaussdb:Test@123@127.0.0.1:5432/postgres?sslmode=disable",
			},
			want: "postgres://gaussdb:%2A%2A%2A%2A%2A%2A@127.0.0.1:5432/postgres?sslmode=disable",
		},
		{
			name: "localhost:55432",
			args: args{
				dsn: "postgres://userDsn:xxxxx@localhost:55432/?sslmode=disabled",
			},
			want: "postgres://userDsn:%2A%2A%2A%2A%2A%2A@localhost:55432/?sslmode=disabled",
		},
		{
			name: "127.0.0.1:5432",
			args: args{
				dsn: "user=xxx password=xxx host=127.0.0.1 port=5432 dbname=postgres sslmode=disable",
			},
			want: "user=xxx%20password=xxx%20host=127.0.0.1%20port=5432%20dbname=postgres%20sslmode=disable",
		},
		{
			name: "localhost:1234",
			args: args{
				dsn: "port=1234",
			},

			want: "port=1234",
		},
		{
			name: "example:5432",
			args: args{
				dsn: "host=example",
			},
			want: "host=example",
		},
		{
			name: "xyz",
			args: args{
				dsn: "xyz",
			},
			want: "xyz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShadowDSN(tt.args.dsn); got != tt.want {
				t.Errorf("ShadowDSN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	type args struct {
		a []string
		x string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Contains",
			args: args{
				a: []string{"a", "b"},
				x: "a",
			},
			want: true,
		},
		{
			name: "Not Contains",
			args: args{
				a: []string{"a", "b"},
				x: "c",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.args.a, tt.args.x); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseCSV(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name     string
		args     args
		wantTags []string
	}{
		{
			name:     "parseCSV",
			args:     args{s: "a1=a1,b1=b1"},
			wantTags: []string{"a1=a1", "b1=b1"},
		},
		{
			name:     "nil",
			args:     args{s: ""},
			wantTags: nil,
		},
		{
			name:     ",",
			args:     args{s: ","},
			wantTags: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotTags := parseCSV(tt.args.s); !reflect.DeepEqual(gotTags, tt.wantTags) {
				t.Errorf("parseCSV() = %v, want %v", gotTags, tt.wantTags)
			}
		})
	}
}

func Test_parseVersionSem1(t *testing.T) {
	type args struct {
		versionString string
	}
	tests := []struct {
		name    string
		args    args
		want    semver.Version
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "MogDB Kernel V500R001C20",
			args: args{versionString: "PostgreSQL 9.2.4 (MogDB   Kernel V500R001C20 build 9eff8f60) compiled at 2021-09-24 10:10:25 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: semver.Version{
				Major: 500,
				Minor: 1,
				Patch: 20,
				Pre:   nil,
				Build: nil,
			},
			wantErr: assert.NoError,
		},
		{
			name: "GaussDB Kernel V500R001C20",
			args: args{versionString: "PostgreSQL 9.2.4 (GaussDB Kernel V500R001C20 build 9eff8f60) compiled at 2021-09-24 10:10:25 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: semver.Version{
				Major: 500,
				Minor: 1,
				Patch: 20,
				Pre:   nil,
				Build: nil,
			},
			wantErr: assert.NoError,
		},
		{
			name: "GaussDB Kernel 505.0.0.SPC0500",
			args: args{versionString: "gaussdb (GaussDB Kernel 505.0.0.SPC0500 build 9eff8f60) compiled at 2021-09-24 10:10:25 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: semver.Version{
				Major: 505,
				Minor: 0,
				Patch: 0,
				Pre:   nil,
				Build: nil,
			},
			wantErr: assert.NoError,
		},
		{
			name: "og_2.0.0",
			args: args{versionString: "PostgreSQL 9.2.4 (openGauss 2.0.0 build 78689da9) compiled at 2021-03-31 21:04:03 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: semver.Version{
				Major: 2,
				Minor: 0,
				Patch: 0,
				Pre:   nil,
				Build: nil,
			},
			wantErr: assert.NoError,
		},
		{
			name: "(openGauss 1.0.0 build",
			args: args{versionString: "(openGauss 1.0.0 build"},
			want: semver.Version{
				Major: 1,
				Minor: 0,
				Patch: 0,
				Pre:   nil,
				Build: nil,
			},
			wantErr: assert.NoError,
		},
		{
			name:    "aaaaa",
			args:    args{versionString: "aaaa"},
			want:    semver.Version{},
			wantErr: assert.Error,
		},
		{
			name: "Uqbar Kernel 1.1.0",
			args: args{versionString: "(Uqbar 1.1.0 build 3eddf83c) compiled at 2022-09-27 00:49:27 commit 0 last mr   on aarch64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: semver.Version{
				Major: 1,
				Minor: 1,
				Patch: 0,
				Pre:   nil,
				Build: nil,
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVersionSem(tt.args.versionString)
			if !tt.wantErr(t, err, fmt.Sprintf("parseVersionSem(%v)", tt.args.versionString)) {
				return
			}
			assert.Equalf(t, tt.want, got, "parseVersionSem(%v)", tt.args.versionString)
		})
	}
}

func Test_parseVersion(t *testing.T) {
	type args struct {
		versionString string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "1.0.0",
			args: args{versionString: "(openGauss 1.0.0 build 5ed8dc17) compiled at 2020-09-15 18:04:09 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 8.2.0, 64-"},
			want: "1.0.0",
		},
		{
			name: "1.0.1",
			args: args{versionString: "(openGauss 1.0.1 build 89d339ca) compiled at 2020-12-21 11:12:55 commit 0 last mr   on aarch64-unknown-linux-gnu, compiled by g++ (GCC) 8.2.0, 64-bit"},
			want: "1.0.1",
		},
		{
			name: "1.1.0",
			args: args{versionString: "PostgreSQL 9.2.4 (openGauss 1.1.0 build 392c0438) compiled at 2020-12-31 20:07:42 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: "1.1.0",
		},
		{
			name: "MogDB_1.1.0",
			args: args{versionString: "PostgreSQL 9.2.4 (MogDB 1.1.0 build fffb972f) compiled at 2021-03-08 15:01:26 commit 0 last mr   on aarch64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: "1.1.0",
		},
		{
			name: "og_2.0.0",
			args: args{versionString: "PostgreSQL 9.2.4 (openGauss 2.0.0 build 78689da9) compiled at 2021-03-31 21:04:03 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: "2.0.0",
		},
		{
			name: "GaussDB Kernel V500R001C20",
			args: args{versionString: "PostgreSQL 9.2.4 (GaussDB Kernel V500R001C20 build 9eff8f60) compiled at 2021-09-24 10:10:25 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: "500.1.20",
		},
		{
			name: "Vastbase",
			args: args{versionString: "PostgreSQL 9.2.4 (Vastbase G100 V2.2 (Build 5.83.5339)) compiled at 2022-02-18 06:19:51 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: "2.2",
		},
		{
			name: "Uqbar 1.1.0",
			args: args{versionString: "(Uqbar 1.1.0 build 3eddf83c) compiled at 2022-09-27 00:49:27 commit 0 last mr   on aarch64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit"},
			want: "1.1.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseVersion(tt.args.versionString)
			if got != tt.want {
				t.Errorf("parseVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
