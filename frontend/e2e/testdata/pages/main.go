package main

import (
	"encoding/json"
	"log"
	"os"
	"strconv"

	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/models"
)

func main() {
	pages := make(map[string]string)
	host := models.ProxyHost{Enabled: true, DomainNames: json.RawMessage(`["preview.test"]`)}
	for _, route := range caddy.BuildErrorRoutes([]models.ProxyHost{host}, "preview") {
		handler := route.Handle[0]
		pages["proxy-"+strconv.Itoa(handler.StatusCode)] = handler.Body
	}
	for _, code := range []int{200, 404, 410, 503, 451} {
		route, err := caddy.BuildDeadHostRoute(&models.DeadHost{
			DomainNames: json.RawMessage(`["preview.test"]`), StatusCode: code,
		}, "preview")
		if err != nil {
			log.Fatal(err)
		}
		pages["parked-"+strconv.Itoa(code)] = route.Handle[0].Body
	}
	if err := json.NewEncoder(os.Stdout).Encode(pages); err != nil {
		log.Fatal(err)
	}
}
