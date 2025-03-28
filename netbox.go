package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	_ "github.com/joho/godotenv/autoload"

	nb "github.com/sapcc/netappsd/internal/pkg/netbox"
)

func main() {
	netboxHost := os.Getenv("NETBOX_HOST")
	netboxToken := os.Getenv("NETBOX_TOKEN")

	if !strings.HasPrefix(netboxHost, "http://") && !strings.HasPrefix(netboxHost, "https://") {
		netboxHost = "https://" + netboxHost
	}

	client, err := nb.NewClient(netboxHost, netboxToken)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	filers, err := client.GetFilers(ctx, "eu-de-1", "manila")
	if err != nil {
		panic(err)
	}
	for _, f := range filers {
		fmt.Printf("%s: Host=%s IpAddr=%s Status=%s AZ=%s\n", f.Name, f.Host, f.Ip, f.Status, f.AvailabilityZone)
	}
	fmt.Printf("found %d filers\n", len(filers))
}
