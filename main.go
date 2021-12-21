package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"
)

var hosts = []string{
	"google.de",
	"google.com",
	"facebook.com",
	"twitter.com",
	"github.com",
	"microsoft.com",
	"akamai.com",
	"apple.com",
	"reddit.com",
	"wired.com",
	"mozilla.org",
	"kernel.org",
	"coredns.io",
	"kubernetes.io",
	"docker.com",
	"cockroachlabs.com",
	"news.ycombinator.com",
	"holidaycheck.com",
	"holidaycheck.de",
	"holidaycheck.at",
	"holidaycheck.ch",
	"iocrunch.com",
}

const dnsCallTimeout = 10 * time.Second

type resolveStat struct {
	d        time.Duration
	timedOut bool
}

func probeLoop(ctx context.Context, resultsChan chan resolveStat) {
	var wg sync.WaitGroup
	for _, host := range hosts {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			stat := resolveStat{}
			ctx, cancel := context.WithTimeout(ctx, dnsCallTimeout)
			defer cancel()
			start := time.Now()
			var err error
			_, err = net.DefaultResolver.LookupHost(ctx, host)
			stat.d = time.Since(start)
			if err == context.Canceled {
				stat.timedOut = true
			}
			resultsChan <- stat
		}(host)
	}
	wg.Wait()
}

func handleStats(ctx context.Context, rstat chan resolveStat) {
	var dnsCalls int
	var timeouts int
	var min, max time.Duration
	var below50ms, below500ms, below1s, above1s, above5s int
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()

	for {
		select {
		case stat := <-rstat:
			dnsCalls++
			if stat.timedOut {
				timeouts++
				continue
			}
			if stat.d > max {
				max = stat.d
			}
			if stat.d < min {
				min = stat.d
			}
			switch {
			case stat.d < 50*time.Millisecond:
				below50ms++
			case stat.d < 500*time.Millisecond:
				below500ms++
			case stat.d < 1*time.Second:
				below1s++
			case stat.d >= 5*time.Second:
				above5s++
			case stat.d >= 1*time.Second:
				above1s++
			}
		case <-t.C:
			fmt.Printf("REPORT at %s\n", time.Now())
			fmt.Printf("REPORT: calls: '%d'; timeouts (at %s): '%d'\n", dnsCalls, dnsCallTimeout, timeouts)
			fmt.Printf("REPORT: below 50ms: '%d'; below 500ms: '%d', below 1s: '%d'; above 1s: '%d'; above 5s: '%d'\n",
				below50ms, below500ms, below1s, above1s, above5s)
			fmt.Printf("REPORT: latency min: '%s'; latency max: '%s'\n", min, max)
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())

	resultsChan := make(chan resolveStat)
	go handleStats(ctx, resultsChan)

	t := time.NewTicker(10 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			probeLoop(ctx, resultsChan)
		case <-sigChan:
			cancel()
			return
		}
	}
}
