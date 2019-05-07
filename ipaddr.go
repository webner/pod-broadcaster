package main

import (
	"fmt"
	"os"
	"time"

	"path/filepath"

	_ "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var reqServiceList chan int
var ipList chan []string

func FetchServiceList(ns, name string) {
	var nextUpdate = time.Now()
	var cachedSrvs []string

	reqServiceList = make(chan int)
	ipList = make(chan []string)

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {

		kubeconfig := filepath.Join(homeDir(), ".kube", "config")
		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	for {
		<-reqServiceList

		if nextUpdate.Before(time.Now()) {
			fmt.Println("updating service targets:")

			cachedSrvs = nil
			endpoints, err := clientset.CoreV1().Endpoints(ns).Get(name, metav1.GetOptions{})
			if err != nil {
				panic(err.Error())
			}

			if len(endpoints.Subsets) > 0 {
				// TODO use all subsets and target port to determine correct ip addresses
				ss := endpoints.Subsets[0]
				for _, ip := range ss.Addresses {
					fmt.Printf("ip: %s \n", ip.IP)
					cachedSrvs = append(cachedSrvs, ip.IP)
				}

			}

			nextUpdate = time.Now().Add(time.Duration(10) * time.Second)
		}

		ipList <- cachedSrvs
	}
}

func GetServiceList() []string {
	reqServiceList <- 1
	return (<-ipList)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
