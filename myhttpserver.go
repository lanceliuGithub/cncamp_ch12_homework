package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
  "math/rand"
	"net/http"
	"os"
	"os/signal"
	"time"
	"path/filepath"
	"syscall"

  "github.com/prometheus/client_golang/prometheus/promhttp"
  //"myhttpserver/metrics"
  "github.com/lanceliuGithub/cncamp_ch10_homework/metrics"
)

type MyConfig struct {
	Server  Server `json:"server"`
	Log     Log    `json:"log"`
}
type Server struct {
	Host string `json:"host"`
	Port string `json:"port"`
}
type Log struct {
	Enable              bool `json:"enable"`
	EnableRequestHeader bool `json:"request_header"`
  EnableResponseTime  bool `json:"reponse_time"`
}

const defaultConfigFile = "config.json"
var myConf *MyConfig
var startTime time.Time

func main() {
	startTime = time.Now()
  metrics.Register()

	// Parse Command-Line Flags
	confFilepathPtr := flag.String("c", defaultConfigFile, "Specify an alternative config file")
	flag.Parse()
	confFilepath := *confFilepathPtr

	// Load Config
	myConf = new(MyConfig)
	loadConfig(confFilepath)

	// Register http handlers
	rootHandler := wrapHandlerWithLogging(http.HandlerFunc(handleRoot))
	http.Handle("/", rootHandler)

	healthzHandler := wrapHandlerWithLogging(http.HandlerFunc(handleHealthz))
	http.Handle("/healthz", healthzHandler)

  //metricsHandler := wrapHandlerWithLogging(http.HandlerFunc(promhttp.Handler()))
  //http.Handle("/metrics", metricsHandler)
  http.Handle("/metrics", promhttp.Handler())

	// Start HTTP server
	go func() {
		printOSEnvVersion()

		// Startup consuming time 5 sec
		for i:=0; i<5; i++ {
			log.Print("Server is starting ...")
			time.Sleep(time.Second)
		}

		serverHost := myConf.Server.Host
		serverPort := myConf.Server.Port
		log.Print("HTTP Server started, listening on " + serverHost + ":" + serverPort)
		if err := http.ListenAndServe(serverHost + ":" + serverPort, nil); err != http.ErrServerClosed {
			log.Fatalf("HTTP server crashed: %v", err)
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(
		signalChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGTERM,
	)

	<-signalChan
	log.Print("OS Interrupt -- HTTP server shutting down")

	go func() {
		<-signalChan
		log.Fatal("OS kill -- Termination")
	}()

	defer os.Exit(0)
	return
}

func loadConfig(confFilename string) {

	// Default config file
	var confFilepath string
	if confFilename == defaultConfigFile {
		ExecPath, _ := os.Executable()
		confFilepath = filepath.Dir(ExecPath) + "/" + confFilename
	} else {
		confFilepath = confFilename
	}

	configFile, err := os.Open(confFilepath)

	if err != nil {
		log.Fatalf("Config file <%s> NOT found: %v", confFilename, err)
	}
	defer configFile.Close()

	configBytes, err := ioutil.ReadAll(configFile)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(configBytes, &myConf);
	if err != nil {
		panic(err)
	}
}

func randInt(min int, max int) int {
  rand.Seed(time.Now().UTC().UnixNano())
  return min + rand.Intn(max-min)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

  timer := metrics.NewTimer()
  defer timer.ObserveTotal()

  delayMillisecs := randInt(10,2000)
  delay := time.Millisecond * time.Duration(delayMillisecs)
  time.Sleep(delay)

	logEnabled := myConf.Log.Enable
	logResponseTimeEnabled := myConf.Log.EnableResponseTime
	if logEnabled && logResponseTimeEnabled {
    log.Printf("wait %d ms\n", delayMillisecs)
  }

	printRequestHeaders(w, r)
}

// 当访问 /healthz 时，状态码200，返回success字样
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	cost:=time.Since(startTime)

	if cost > 10 * time.Second {
		fmt.Fprintf(w, "success\n")
		printRequestHeaders(w, r)
	} else {
		http.Error(w, "failed", http.StatusInternalServerError)
	}
}

// 读取当前系统的环境变量中的 VERSION 配置
func printOSEnvVersion() {
	var v string
	if v = os.Getenv("VERSION"); v == "" {
		v = "Unknown"
	}
	log.Printf("MyHTTPServer Version: %s\n", v)
}

// 接收客户端 request，并将 request 中带的 header 写入 response header
func printRequestHeaders(w http.ResponseWriter, r *http.Request) {
	logEnabled := myConf.Log.Enable
	logRequestHeaderEnabled := myConf.Log.EnableRequestHeader
	//log.Printf("logEnabled: " + strconv.FormatBool(logEnabled))

	if ! logEnabled || ! logRequestHeaderEnabled {
		return
	}

	log.Printf("Request Headers (Total of %d):\n", len(r.Header))
	for k, v := range r.Header {
		log.Printf("==> %s: %s\n", k, v)
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Server 端记录访问日志包括客户端 IP，HTTP 返回码，输出到 server 端的标准输出
func wrapHandlerWithLogging(wrappedHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logEnabled := myConf.Log.Enable

		if logEnabled {
			log.Printf("--> %s %s", req.Method, req.URL.Path)
		}

		lrw := newLoggingResponseWriter(w)
		wrappedHandler.ServeHTTP(lrw, req)

		if logEnabled {
			statusCode := lrw.statusCode
			log.Printf("<-- %d %s", statusCode, http.StatusText(statusCode))
		}
	})
}
