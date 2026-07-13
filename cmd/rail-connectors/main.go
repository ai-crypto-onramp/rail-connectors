// Command rail-connectors runs the simplified rail connectors HTTP service.
package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/ai-crypto-onramp/rail-connectors/internal/dummy"
	"github.com/ai-crypto-onramp/rail-connectors/internal/server"
)

func main() {
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = ":8080"
	}
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}
	rail := os.Getenv("RAIL_FAMILY")
	if rail == "" {
		rail = "dummy"
	}
	srv := server.New(server.Config{
		Addr:          addr,
		Rail:          rail,
		WebhookSecret: os.Getenv("WEBHOOK_SECRET"),
		Ready:         true,
	})
	log.Printf("rail-connectors listening on %s (rail=%s)", addr, rail)
	if err := http.ListenAndServe(addr, srv.Mux()); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
