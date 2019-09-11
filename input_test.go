package main

import (
	fmt "fmt"
	"math/rand"
	"strconv"
	"time"

	riemanngo "github.com/riemann/riemann-go-client"
)

func sendTestEvent(addr string) (err error) {
	c := riemanngo.NewTCPClient(addr, 1*time.Second)
	if err = c.Connect(); err != nil {
		return
	}

	r, err := riemanngo.SendEvents(c, &[]riemanngo.Event{{
		Service:     "foo",
		Host:        "bar",
		Description: "baz",
	}})

	if err != nil {
		return fmt.Errorf("Unable to send event: %s", err)
	}

	if r.GetOk() != true {
		return fmt.Errorf("Not Ok response")
	}

	c.Close()
	return
}

func getTestInput() (i *input, c *inputCfg, err error) {
	c = &inputCfg{
		Name: "test",
	}

	for port := 0; port < 50; port++ {
		c.Listen = "127.0.0.1:" + strconv.Itoa(rand.Intn(20000)+20000)
		if i, err = newInput(c); err == nil {
			return
		}
	}

	return
}

// func Test_Input(t *testing.T) {
// 	ch := make(chan []*Event, 10)

// 	i, cf, err := getTestInput()
// 	assert.Nil(t, err)
// 	i.addChannel("test", ch)

// 	err = sendTestEvent(cf.Listen)
// 	assert.Nil(t, err)

// 	assert.Equal(t, "receivedBatches 1 receivedEvents 1 dropped 0", i.getStats())
// 	i.Close()

// 	evT := &Event{
// 		Service:     pb.String("foo"),
// 		Host:        pb.String("bar"),
// 		Description: pb.String("baz"),
// 	}

// 	batch, ok := <-ch
// 	assert.True(t, ok)
// 	assert.Equal(t, 1, len(batch))

// 	ev := batch[0]
// 	assert.Equal(t, evT.Service, ev.Service)
// 	assert.Equal(t, evT.Host, ev.Host)
// 	assert.Equal(t, evT.Description, ev.Description)
// }
