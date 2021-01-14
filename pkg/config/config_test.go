package config

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

type args struct {
	reader io.ReadCloser
}

type faultyReader int

func (faultyReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("test error")
}

func ReaderFrom(conf interface{}) io.ReadCloser {
	b, _ := yaml.Marshal(conf)
	return ioutil.NopCloser(bytes.NewReader(b))
}

func TestNew(t *testing.T) {
	var validConfig = &ForwardProxyConfig{
		Proxy:      Proxy{Server: "localhost", Port: 1994, BufferSizes: BufferSizes{Read: 1024, Write: 1024}, Limits: Limits{MaxConnsPerIP: 0, MaxBodySize: 1024}, Timeouts: Timeouts{Read: "30s", Write: "30s", Connect: "30s"}},
		Monitoring: Monitoring{Port: 2000},
	}

	var invalidProxyPort = &ForwardProxyConfig{
		Proxy:      Proxy{Server: "localhost", Port: 1, BufferSizes: BufferSizes{Read: 1024, Write: 1024}, Limits: Limits{MaxConnsPerIP: 0, MaxBodySize: 1024}, Timeouts: Timeouts{Read: "30s", Write: "30s", Connect: "30s"}},
		Monitoring: Monitoring{Port: 2000},
	}

	var invalidMetricPort = &ForwardProxyConfig{
		Proxy:      Proxy{Server: "localhost", Port: 18080, BufferSizes: BufferSizes{Read: 1024, Write: 1024}, Limits: Limits{MaxConnsPerIP: 0, MaxBodySize: 1024}, Timeouts: Timeouts{Read: "30s", Write: "30s", Connect: "30s"}},
		Monitoring: Monitoring{Port: 1},
	}

	var invalidConnectTime = &ForwardProxyConfig{
		Proxy:      Proxy{Server: "localhost", Port: 1994, Timeouts: Timeouts{Read: "30s", Write: "30s", Connect: "40"}},
		Monitoring: Monitoring{Port: 2000},
	}

	var invalidWriteTime = &ForwardProxyConfig{
		Proxy:      Proxy{Server: "localhost", Port: 1994, Timeouts: Timeouts{Read: "30s", Write: "40", Connect: "30s"}},
		Monitoring: Monitoring{Port: 2000},
	}

	var invalidReadTime = &ForwardProxyConfig{
		Proxy:      Proxy{Server: "localhost", Port: 1994, Timeouts: Timeouts{Read: "40", Write: "30s", Connect: "30s"}},
		Monitoring: Monitoring{Port: 2000},
	}

	var minimalConfig = &ForwardProxyConfig{
		Proxy:      Proxy{Server: "localhost", Port: 1994, Timeouts: Timeouts{Read: "40s", Write: "30s", Connect: "30s"}},
		Monitoring: Monitoring{Port: 2000},
	}

	var defaultsFilled = &ForwardProxyConfig{
		Proxy:      Proxy{Server: "localhost", Port: 1994, Timeouts: Timeouts{Read: "40s", Write: "30s", Connect: "30s"}, Limits: Limits{MaxConnsPerIP: 0, MaxBodySize: 4 * 1024 * 1024}, BufferSizes: BufferSizes{Read: 4096, Write: 4096}},
		Monitoring: Monitoring{Port: 2000},
	}

	invalidYaml := &struct {
		Proxy struct {
			Server int    `yaml:"Server"`
			Port   string `yaml:"Port"`
		} `yaml:"Proxy"`
	}{struct {
		Server int    "yaml:\"Server\""
		Port   string "yaml:\"Port\""
	}{12312312, "superPort"}}

	tests := []struct {
		name string
		args args

		want        *ForwardProxyConfig
		expectErr   bool
		wantMessage string
	}{
		{name: "valid Config", args: args{reader: ReaderFrom(validConfig)}, want: validConfig, expectErr: false},
		{name: "fill defaults config", args: args{reader: ReaderFrom(minimalConfig)}, want: defaultsFilled, expectErr: false},
		{name: "invalid monitoring port", args: args{reader: ReaderFrom(invalidMetricPort)}, expectErr: true, wantMessage: "monitoring port is not within valid range"},
		{name: "invalid proxy port", args: args{reader: ReaderFrom(invalidProxyPort)}, expectErr: true, wantMessage: "proxy port is not within valid range"},
		{name: "invalid connect time", args: args{reader: ReaderFrom(invalidConnectTime)}, expectErr: true, wantMessage: "missing unit in duration"},
		{name: "invalid write time", args: args{reader: ReaderFrom(invalidWriteTime)}, expectErr: true, wantMessage: "missing unit in duration"},
		{name: "invalid read time", args: args{reader: ReaderFrom(invalidReadTime)}, expectErr: true, wantMessage: "missing unit in duration"},
		{name: "invalid config", args: args{reader: ReaderFrom(invalidYaml)}, expectErr: true, wantMessage: "cannot unmarshal"},
		{name: "faulty reader", args: args{reader: ioutil.NopCloser(faultyReader(0))}, expectErr: true, wantMessage: "test error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.reader)

			if tt.expectErr {
				assert.Error(t, err, "should throw err")
				assert.Contains(t, err.Error(), tt.wantMessage, "Did not throw expected error")
			} else {
				assert.NoError(t, err, "should not throw error")
				assert.EqualValues(t, tt.want, got, "did not receive expected value")
			}
		})
	}
}
