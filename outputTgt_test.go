package main

import (
	"sync"
	"sync/atomic"
	"testing"
)

// var testCfg = &outputCfg{
// 	Name:              "test",
// 	Type:              "riemann",
// 	Algo:              "hash",
// 	HashFields:        []string{"host"},
// 	CarbonFields:      []string{"service", "host", "description"},
// 	CarbonValue:       "any",
// 	TimeoutConnect:    duration{1 * time.Second},
// 	ReconnectInterval: duration{1 * time.Second},
// 	BufferSize:        50,
// 	BatchSize:         1,
// 	BatchTimeout:      duration{1 * time.Second},
// }

// func Test_outputRiemann(t *testing.T) {
// 	cfg.StatsInterval.Duration = 50 * time.Millisecond

// 	ch := make(chan []*Event, 10)
// 	i, cf, err := getTestInput()
// 	assert.Nil(t, err)
// 	i.addChannel("test", ch)

// 	testCfg.Targets = []string{cf.Listen}
// 	test := func(algo string) {
// 		testCfg.Algo = algo
// 		o, err := newOutput(testCfg)
// 		assert.Nil(t, err)

// 		evT := &Event{
// 			Service:     pb.String("foo"),
// 			Host:        pb.String("bar"),
// 			Description: pb.String("baz"),
// 		}

// 		time.Sleep(100 * time.Millisecond)
// 		o.chanIn <- []*Event{evT}

// 		batch, ok := <-ch
// 		assert.True(t, ok)
// 		assert.Equal(t, 1, len(batch))

// 		ev := batch[0]
// 		assert.Equal(t, evT.Service, ev.Service)
// 		assert.Equal(t, evT.Host, ev.Host)
// 		assert.Equal(t, evT.Description, ev.Description)

// 		o.Close()
// 	}

// 	for _, a := range outputAlgoMapRev {
// 		test(a)
// 	}

// 	i.Close()
// }

// func Test_outputCarbon(t *testing.T) {
// 	var (
// 		addr string
// 		err  error
// 		l    net.Listener
// 	)

// 	ch := make(chan string, 10)

// 	for i := 0; i < 50; i++ {
// 		addr = "127.0.0.1:" + strconv.Itoa(20000+rand.Intn(20000))
// 		if l, err = listen(addr); err == nil {
// 			break
// 		}
// 	}
// 	assert.Nil(t, err)

// 	accept := func() {
// 		c, err := l.Accept()
// 		assert.Nil(t, err)

// 		bio := bufio.NewReader(c)
// 		row, err := bio.ReadString('\n')
// 		assert.Nil(t, err)

// 		ch <- strings.TrimSpace(row)
// 	}

// 	go accept()

// 	testCfg.Type = "carbon"
// 	testCfg.Algo = "hash"
// 	testCfg.Targets = []string{addr}
// 	o, err := newOutput(testCfg)
// 	assert.Nil(t, err)

// 	evT := &Event{
// 		Service:      pb.String("foo"),
// 		Host:         pb.String("bar"),
// 		Description:  pb.String("baz"),
// 		MetricSint64: pb.Int64(123),
// 		Time:         pb.Int64(12345),
// 	}

// 	time.Sleep(100 * time.Millisecond)
// 	o.chanIn <- []*Event{evT}

// 	row := <-ch
// 	assert.Equal(t, "foo.bar.baz 123 12345", row)

// 	time.Sleep(100 * time.Millisecond)
// 	o.Close()
// 	l.Close()
// }

func Benchmark_Atomic(b *testing.B) {
	var a int32
	for i := 0; i < b.N; i++ {
		atomic.AddInt32(&a, 1)
	}
}

func Benchmark_Mtx(b *testing.B) {
	var a int32
	var m sync.Mutex
	for i := 0; i < b.N; i++ {
		m.Lock()
		a++
		m.Unlock()
	}
}

func Benchmark_Make(b *testing.B) {
	m := map[int]uint64{
		0: 0,
		1: 1,
		2: 2,
		3: 3,
		4: 4,
		5: 5,
		6: 6,
		7: 7,
	}

	var mx sync.Mutex
	for i := 0; i < b.N; i++ {
		mx.Lock()
		a := make([]uint64, int32(len(m)))
		i := 0
		for _, v := range m {
			a[i] = v
		}
		mx.Unlock()
	}
}
