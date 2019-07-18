package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
)

var config struct {
	targetNamespace string
	targetService   string
	targetScheme    string
	targetPort      int
	port            int
}

var BuildDate = "unknown"
var Version = "unknown"

var client http.Client

func getEnvOrDefault(name, defaultValue string) string {
	v := os.Getenv(name)
	if v != "" {
		return v
	}
	return defaultValue
}

func main() {

	fmt.Printf("pod-broadcaster %v\n", Version)
	fmt.Printf("%v\n", BuildDate)
	fmt.Println("------------------------------")

	fmt.Printf("GOMAXPROCS: %v\n", runtime.GOMAXPROCS(0))
	targetPort, _ := strconv.Atoi(getEnvOrDefault("TARGET_PORT", "8080"))
	port, _ := strconv.Atoi(getEnvOrDefault("LISTEN_PORT", "8080"))

	flag.StringVar(&config.targetService, "targetService", getEnvOrDefault("TARGET_SERVICE", ""), "target service to broadcast.")
	flag.StringVar(&config.targetNamespace, "targetNamespace", getEnvOrDefault("TARGET_NAMESPACE", "default"), "the namespace of the target service.")
	flag.IntVar(&config.targetPort, "targetPort", targetPort, "target port of the service.")
	flag.StringVar(&config.targetScheme, "targetScheme", getEnvOrDefault("TARGET_SCHEME", "http"), "target scheme of the service (http or https).")
	flag.IntVar(&config.port, "port", port, "listening port of the broadcaster.")
	flag.Parse()

	fmt.Printf("Target service: %s\n", config.targetService)
	fmt.Printf("Target namespace: %s\n", config.targetNamespace)
	fmt.Printf("Target port: %d\n", targetPort)
	fmt.Printf("Listen port: %d\n", port)

	go FetchServiceList(config.targetNamespace, config.targetService)
	go httpServer()

	fmt.Println("Press ctrl-C to exit")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ch:
		log.Println("Stop signal received")
	}
	fmt.Println("Exiting")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	var health struct {
		Status string
	}

	health.Status = "OK"

	result, _ := json.Marshal(health)
	w.Write(result)
	w.Write([]byte("\n"))
}

type result struct {
	Status int
	Method string
	Host   string
	URL    string
	Value  interface{}
}

type resultList struct {
	Result []result
}

func aggregateHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

	var aggregatedResult resultList

	fmt.Printf("handle %v\n", r.URL.String())

	srvs := GetServiceList()

	ch := make(chan result, len(srvs))

	defer r.Body.Close()
	body, _ := ioutil.ReadAll(r.Body)

	for _, v := range srvs {
		go func(target string) {

			host := target

			if config.targetPort != 80 {
				host = target + ":" + strconv.Itoa(config.targetPort)
			}

			url := *r.URL
			url.Scheme = config.targetScheme
			url.Host = host
			fmt.Printf(" %v %v\n", r.Method, url.String())

			req, err := http.NewRequest(r.Method, url.String(), bytes.NewReader(body))
			req.Header = r.Header
			resp, err := client.Do(req)

			r := result{resp.StatusCode, r.Method, target, url.String(), nil}

			if err != nil {
				r.Value = err.Error
				ch <- r
				return
			}

			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)

			var plainData interface{}
			err = json.Unmarshal(body, &plainData)
			if err != nil {
				plainData = body
			}
			r.Value = plainData
			ch <- r
		}(v)

	}

	for i := 0; i < len(srvs); i++ {
		data := <-ch
		aggregatedResult.Result = append(aggregatedResult.Result, data)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	result, err := json.Marshal(aggregatedResult)
	if err != nil {
		panic(err.Error())
	}
	w.Write(result)
	w.Write([]byte("\n"))
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pod-broadcaster "))
	w.Write([]byte(Version))
}

func httpServer() {
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/version", versionHandler)
	http.HandleFunc("/", aggregateHandler)
	http.ListenAndServe(net.JoinHostPort("", strconv.Itoa(config.port)), nil)
}
