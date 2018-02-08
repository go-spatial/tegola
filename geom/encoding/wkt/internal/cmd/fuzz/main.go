package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/geom/encoding/wkt"
	"github.com/terranodo/tegola/geom/encoding/wkt/internal/cmd/fuzz/fuzz"
)

type panicReport struct {
	i int
	g geom.Geometry
	r interface{}
}

type wt struct {
	i  int
	gt string
}

func worker(ctx context.Context, wg *sync.WaitGroup, id chan int, panicChan chan panicReport, workOn chan wt) {
	var i int
	var g geom.Geometry

	defer func() {
		if r := recover(); r != nil {
			panicChan <- panicReport{
				i: i,
				g: g,
				r: r,
			}
		}
		wg.Done()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case i := <-id:
			g = fuzz.GenGeometry()
			gt := fmt.Sprintf("%T", g)
			workOn <- wt{
				i:  i,
				gt: gt,
			}
			wkt.Encode(g)
		}
	}
}

func main() {
	var wg sync.WaitGroup
	var typeCount = make(map[string]uint64)
	var panicrpts []panicReport

	ctx, cancel := context.WithCancel(context.Background())
	ctx1, cancel1 := context.WithCancel(context.Background())
	idchan := make(chan int)
	panicchan := make(chan panicReport)
	workChan := make(chan wt)

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go worker(ctx, &wg, idchan, panicchan, workChan)
	}
	var wg1 sync.WaitGroup
	wg1.Add(1)
	go func(ctx context.Context) {
		defer func() {
			wg1.Done()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case r := <-panicchan:
				panicrpts = append(panicrpts, r)
			}

		}
	}(ctx1)
	wg1.Add(1)
	go func(ctx context.Context) {
		defer func() {
			wg1.Done()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case w := <-workChan:
				typeCount[w.gt]++
				fmt.Printf("                                                                                \rLooking at object %010v %v\r", w.i, w.gt)
			}
		}
	}(ctx1)

	for i := 0; i < 100; i++ {
		idchan <- i
	}
	cancel()
	wg.Wait()
	cancel1()
	wg1.Wait()
	log.Println("TypeCount:", typeCount)
	for _, pr := range panicrpts {
		log.Printf("Found Panic at Object %v : %v : %v\n", pr.i, pr.r, pr.g)
	}
}
