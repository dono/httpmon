package main

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"net/http"
	"net/http/httptrace"
	"time"
	"strconv"
	"flag"


	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type stats struct {
	tlsCert             *x509.Certificate

	Start                time.Time
	DNSStart             time.Time
	DNSDone              time.Time
	ConnectStart         time.Time
	ConnectDone          time.Time
	TLSHandshakeStart    time.Time
	TLSHandshakeDone     time.Time
	GotConn              time.Time
	GotFirstResponseByte time.Time
	Finish               time.Time
}

func (s *stats) dnsLookup() time.Duration {
	return s.DNSDone.Sub(s.DNSStart)
}

func (s *stats) tcpConnection() time.Duration {
	return s.ConnectDone.Sub(s.ConnectStart)
}

func (s *stats) tlsHandshake() time.Duration {
	return s.TLSHandshakeDone.Sub(s.TLSHandshakeStart)
}

func (s *stats) serverProcessing() time.Duration {
	return s.GotFirstResponseByte.Sub(s.GotConn)
}

func (s *stats) contentTransfer() time.Duration {
	return s.Finish.Sub(s.GotFirstResponseByte)
}

func (s *stats) ttfb() time.Duration {
	return s.GotFirstResponseByte.Sub(s.Start)
}

type httpStatsCollector struct {
	url string
	timeout int

	dnsLookup        *prometheus.Desc
	tcpConnection    *prometheus.Desc
	tlsHandshake     *prometheus.Desc
	serverProcessing *prometheus.Desc
	contentTransfer  *prometheus.Desc
	ttfb             *prometheus.Desc
}

func (c *httpStatsCollector) visit() (stats, *http.Response, error) {
	var s stats
	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			s.DNSStart = time.Now()
		},
		DNSDone: func(ddi httptrace.DNSDoneInfo) {
			s.DNSDone = time.Now()
		},
		ConnectStart: func(_, _ string) {
			s.ConnectStart = time.Now()
		},
		ConnectDone: func(_, addr string, _ error) {
			s.ConnectDone = time.Now()
		},
		TLSHandshakeStart: func() {
			s.TLSHandshakeStart = time.Now()
		},
		TLSHandshakeDone: func(cs tls.ConnectionState, err error) {
			s.TLSHandshakeDone = time.Now()
			if err == nil {
				s.tlsCert = cs.PeerCertificates[0] // End Entity証明書のみ対応
			}
		},
		GotConn: func(_ httptrace.GotConnInfo) {
			s.GotConn = time.Now()
		},
		GotFirstResponseByte: func() {
			s.GotFirstResponseByte = time.Now()
		},
	}

	req, err := http.NewRequest("GET", c.url, nil)
	if err != nil {
		log.Fatalf("Request generation error: %s", err)
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		Timeout: time.Duration(c.timeout) * time.Second,
	}

	s.Start = time.Now()
	resp, err := client.Do(req)
	s.Finish = time.Now()
	if err != nil {
		return s, resp, err
	}

	return s, resp, nil
}

func newHTTPStatsCollector(url string, timeout int) *httpStatsCollector {
	return &httpStatsCollector{
		url: url,
		timeout: timeout,

		dnsLookup: prometheus.NewDesc(
			"dns_lookup_time",
			"A gauge of the DNS lookup durations(ms)",
			[]string{"status_code"},
			nil,
		),
		tcpConnection: prometheus.NewDesc(
			"tcp_handshake_time",
			"A gauge of the TCP handshake duration(ms)",
			[]string{"status_code"},
			nil,
		),
		tlsHandshake: prometheus.NewDesc(
			"tls_handshake_time",
			"A gauge of the TLS handshake duration(ms)",
			[]string{"status_code"},
			nil,
		),
		serverProcessing: prometheus.NewDesc(
			"server_processing_time",
			"A gauge of the server processing duration(ms)",
			[]string{"status_code"},
			nil,
		),
		contentTransfer: prometheus.NewDesc(
			"content_transfer_time",
			"A gauge of the content transfer duration(ms)",
			[]string{"status_code"},
			nil,
		),
		ttfb: prometheus.NewDesc(
			"ttfb",
			"A gauge of the content transfer duration(ms)",
			[]string{"status_code"},
			nil,
		),
	}
}

func (c *httpStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.dnsLookup
	ch <- c.tcpConnection
	ch <- c.tlsHandshake
	ch <- c.serverProcessing
	ch <- c.contentTransfer
	ch <- c.ttfb
}

func (c *httpStatsCollector) Collect(ch chan<- prometheus.Metric) {
	s, resp, err := c.visit()
	if err != nil {
		log.Printf("URL visit error: %s", err)
		return
	}
	defer resp.Body.Close()

	statusCode := "unknown"
	if resp.StatusCode >= 100 && resp.StatusCode <= 199 {
		statusCode = "1xx"
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		statusCode = "2xx"
	}
	if resp.StatusCode >= 300 && resp.StatusCode <= 399 {
		statusCode = "3xx"
	}
	if resp.StatusCode >= 400 && resp.StatusCode <= 499 {
		statusCode = "4xx"
	}
	if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
		statusCode = "5xx"
	}

	dnsLookupMetric, err := prometheus.NewConstMetric(
		c.dnsLookup,
		prometheus.GaugeValue,
		ns2ms(s.dnsLookup()),
		statusCode,
	)
	if err != nil {
		log.Printf("dnsLookup metric generation error: %s", err)
		return
	}
	ch <- dnsLookupMetric

	tcpConnectionMetric, err := prometheus.NewConstMetric(
		c.tcpConnection,
		prometheus.GaugeValue,
		ns2ms(s.tcpConnection()),
		statusCode,
	)
	if err != nil {
		log.Printf("tcpConnection metric generation error: %s", err)
		return
	}
	ch <- tcpConnectionMetric

	tlsHandshakeMetric, err := prometheus.NewConstMetric(
		c.tlsHandshake,
		prometheus.GaugeValue,
		ns2ms(s.tlsHandshake()),
		statusCode,
	)
	if err != nil {
		log.Printf("tlsHandshke metric generation error: %s", err)
		return
	}
	ch <- tlsHandshakeMetric

	serverProcessingMetric, err := prometheus.NewConstMetric(
		c.serverProcessing,
		prometheus.GaugeValue,
		ns2ms(s.serverProcessing()),
		statusCode,
	)
	if err != nil {
		log.Printf("serverProcessing metric generation error: %s", err)
		return
	}
	ch <- serverProcessingMetric

	contentTransferMetric, err := prometheus.NewConstMetric(
		c.contentTransfer,
		prometheus.GaugeValue,
		ns2ms(s.contentTransfer()),
		statusCode,
	)
	if err != nil {
		log.Printf("contentTransfer metric generation error: %s", err)
		return
	}
	ch <- contentTransferMetric

	ttfbMetric, err := prometheus.NewConstMetric(
		c.ttfb,
		prometheus.GaugeValue,
		ns2ms(s.ttfb()),
		statusCode,
	)
	if err != nil {
		log.Printf("ttfb metric generation error: %s", err)
		return
	}
	ch <- ttfbMetric
}

func ns2ms(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func prometheusReqsHandler(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	targetURL := params.Get("target")
	if targetURL == "" {
		http.Error(w, "Target param is missing", http.StatusBadRequest)
		return
	}

	timeout := 10 // default timeout(sec)
		if params.Get("timeout") != "" {
		timeout, err := strconv.Atoi(params.Get("timeout"))
		if err != nil {
			log.Printf("Invalid timeout parameter. Use default timeout: %d", timeout)
		}
	}

	collector := newHTTPStatsCollector(targetURL, timeout)

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}


func main() {
	var (
		addr = flag.String("a", "127.0.0.1:8888", "Listen address")
	)
	flag.Parse()

	http.HandleFunc("/metrics", prometheusReqsHandler)

	log.Printf("Listening on addr %s\n", *addr)
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatalf("Server Listening error: %s", err)
	}
}