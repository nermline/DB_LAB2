// Package bench measures latency and throughput of benchmark operations.
package bench

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

type Result struct {
	Test       string        `json:"test"`
	Variant    string        `json:"variant"`
	Workers    int           `json:"workers"`
	Ops        int           `json:"ops"`
	Min        time.Duration `json:"min_ns"`
	P50        time.Duration `json:"p50_ns"`
	P95        time.Duration `json:"p95_ns"`
	P99        time.Duration `json:"p99_ns"`
	Max        time.Duration `json:"max_ns"`
	Mean       time.Duration `json:"mean_ns"`
	Throughput float64       `json:"ops_per_sec"`
	Note       string        `json:"note,omitempty"`
}

// Sequential runs warmup calls (not measured) and then n measured calls of op(i).
func Sequential(test, variant string, warmup, n int, op func(i int) error) (Result, error) {
	for i := 0; i < warmup; i++ {
		if err := op(n + i); err != nil {
			return Result{}, fmt.Errorf("%s / %s warmup: %w", test, variant, err)
		}
	}
	lat := make([]time.Duration, 0, n)
	start := time.Now()
	for i := 0; i < n; i++ {
		t := time.Now()
		if err := op(i); err != nil {
			return Result{}, fmt.Errorf("%s / %s: %w", test, variant, err)
		}
		lat = append(lat, time.Since(t))
	}
	return summarize(test, variant, 1, lat, time.Since(start)), nil
}

// Concurrent runs op from the given number of workers for the given duration.
func Concurrent(test, variant string, workers int, d time.Duration, op func(worker, i int) error) (Result, error) {
	var (
		mu      sync.Mutex
		all     []time.Duration
		failed  atomic.Value
		wg      sync.WaitGroup
		stopped atomic.Bool
	)
	start := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			var lat []time.Duration
			for i := 0; !stopped.Load(); i++ {
				t := time.Now()
				if err := op(w, i); err != nil {
					failed.Store(err)
					return
				}
				lat = append(lat, time.Since(t))
			}
			mu.Lock()
			all = append(all, lat...)
			mu.Unlock()
		}(w)
	}
	time.Sleep(d)
	stopped.Store(true)
	wg.Wait()
	if err, ok := failed.Load().(error); ok {
		return Result{}, fmt.Errorf("%s / %s: %w", test, variant, err)
	}
	return summarize(test, variant, workers, all, time.Since(start)), nil
}

func summarize(test, variant string, workers int, lat []time.Duration, total time.Duration) Result {
	slices.Sort(lat)
	var sum time.Duration
	for _, l := range lat {
		sum += l
	}
	n := len(lat)
	pct := func(p float64) time.Duration { return lat[min(n-1, int(float64(n)*p))] }
	return Result{
		Test: test, Variant: variant, Workers: workers, Ops: n,
		Min: lat[0], P50: pct(0.50), P95: pct(0.95), P99: pct(0.99), Max: lat[n-1],
		Mean: sum / time.Duration(n), Throughput: float64(n) / total.Seconds(),
	}
}
