package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/sapcc/go-bits/httpapi"
	"github.com/sapcc/go-bits/httpext"
	"github.com/sapcc/go-bits/must"
	"github.com/spf13/cobra"
)

var (
	httpListenAddr string
)

var cmd = &cobra.Command{
	Use:   "netappsd",
	Short: "A simple network application service daemon",
	Long:  `A simple network application service daemon`,
	Run: func(_ *cobra.Command, _ []string) {
		run()
	},
}

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	cmd.Flags().StringVarP(&httpListenAddr, "http-listen-addr", "l", ":8080", "The address on which to listen")
}

func run() {
	log.Print("Starting netappsd")

	netappsd := NewNetAppSD()
	go netappsd.Discover(nil)

	handler := httpapi.Compose(netappsd)
	mux := http.NewServeMux()
	mux.Handle("/", handler)

	ctx := httpext.ContextWithSIGINT(context.Background(), 10*time.Second)
	must.Succeed(httpext.ListenAndServeContext(ctx, httpListenAddr, mux))
}
