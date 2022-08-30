package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func NewResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

var totalRequests = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Number of get requests.",
	},
	[]string{"path"},
)

var responseStatus = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "response_status",
		Help: "Status of HTTP response.",
	},
	[]string{"status"},
)

var httpDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "http_response_time_seconds",
		Help: "Duration of HTTP requests.",
	},
	[]string{"path"},
)

func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()

		timer := prometheus.NewTimer(httpDuration.WithLabelValues(path))
		rw := NewResponseWriter(w)
		next.ServeHTTP(rw, r)

		statusCode := rw.statusCode

		responseStatus.WithLabelValues(strconv.Itoa(statusCode)).Inc()
		totalRequests.WithLabelValues(path).Inc()

		timer.ObserveDuration()
	})
}

var (
	appVersion  = "v0.1"
	localIP     = "1.2.3.4"
	instanceNum = 0
	status      = ""
)

func serverName(w http.ResponseWriter, r *http.Request) {
	name, _ := os.Hostname()
	fmt.Fprintf(w, name)
}

func health(w http.ResponseWriter, r *http.Request) {
	//status := ""
	if r.Method == "POST" {
		status = r.PostFormValue("status")

		if strings.EqualFold(status, "ok") {
			w.WriteHeader(200)
		} else {
			w.WriteHeader(503)
		}
	}

	if strings.EqualFold(status, "failed") {
		w.WriteHeader(503)
	}

	fmt.Fprintf(w, status)
}

func getVersion(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%s\n", appVersion)
}

func getFrontpage(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	clientIP := r.Header.Get("X-Real-Ip")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Forwarded-For")
	}

	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	hostname, _ := os.Hostname()
	fmt.Fprintf(w, "Hello, Go! I'm instance %d running version %s at %s \n\nHostName: %s \nServerIP: %s \nCleintIP: %s\n",
		instanceNum, appVersion, t.Format("2006-01-02 15:04:05"), hostname, localIP, clientIP)
}

func getLocalIPAddress() string {
	addrs, err := net.InterfaceAddrs()

	if err != nil {
		return ""
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return ""
}

func init() {
	prometheus.Register(totalRequests)
	prometheus.Register(responseStatus)
	prometheus.Register(httpDuration)
}

func main() {
	localIP = getLocalIPAddress()
	HOST := os.Getenv("HOST")
	PORT := os.Getenv("PORT")
	addr := fmt.Sprintf("%s:%s", HOST, PORT)

	rand.Seed(time.Now().UTC().UnixNano())
	instanceNum = rand.Intn(1000)

	// Setup router
	r := mux.NewRouter()
	r.Use(prometheusMiddleware)

	// Prometheus endpoint
	r.Path("/metrics").Handler(promhttp.Handler())

	// Serving
	r.Path("/version").HandlerFunc(getVersion)
	r.Path("/health").HandlerFunc(health)
	r.Path("/servername").HandlerFunc(serverName)
	r.PathPrefix("/").HandlerFunc(getFrontpage)

	log.Fatal(http.ListenAndServe(addr, r))
}
