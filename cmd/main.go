package main

import (
	"log"

	"netappsd"
)

func main() {
	cm, err := netappsd.NewConfigMapOutofCluster("test-cm", "kube-monitoring", nil)
	if err != nil {
		log.Println(err)
	}
	log.Println(cm)
}
