package main

import (
	"log"
	"time"

	riemanngo "github.com/riemann/riemann-go-client"
)

func main() {
	c := riemanngo.NewTCPClient("127.0.0.1:1234", 5*time.Second)
	err := c.Connect()
	if err != nil {
		panic(err)
	}

	events := []riemanngo.Event{}
	for i := 0; i < 100; i++ {
		events = append(events, riemanngo.Event{
			Service: "hello",
			Metric:  100,
			Tags:    []string{"hello"},
			Attributes: map[string]string{
				"prefix": "aabbccdd",
			},
		})
	}

	for i := 0; i < 10000; i++ {
		r, err := riemanngo.SendEvents(c, &events)
		if err != nil {
			log.Fatalf("Unable to send events: %s", err)
		}

		if r.GetOk() != true {
			log.Fatal("false")
		}
	}

	//time.Sleep(10 * time.Second)

	c.Close()
}
