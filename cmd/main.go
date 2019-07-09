package main

import (
	"log"

  //atlasWriter "github.com/sapcc/atlas/pkg/writer"
  "netappsd"
)

func main() {
	_, err := netappsd.NewConfigMap("test-cm", "kube-monitoring", nil)
	if err != nil {
		log.Printf("Error: %v", err)
	}
}
