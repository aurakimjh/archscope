// Command bypassclient is an OPTIONAL standalone CAP-4 helper. The automated
// spike already covers CAP-4 with loadgen's last worker (goes_via_proxy=false),
// but when the operator wants to prove bypass detection against a system that
// has a real HTTP proxy configured, this makes direct connections that
// deliberately ignore every proxy setting and prints its identity so it can be
// eyeballed against a probe's observations.
//
// It never reads WinHTTP/WinINET proxy config and dials the target directly,
// which is exactly the behavior a "proxy-noncompliant HTTP client" exhibits.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	target := flag.String("target", "", "target host:port to hit directly, e.g. example.internal:80")
	count := flag.Int("count", 20, "number of direct transactions")
	flag.Parse()
	if *target == "" {
		fmt.Fprintln(os.Stderr, "bypassclient: -target host:port is required")
		os.Exit(2)
	}

	var localPort int
	tr := &http.Transport{
		Proxy: nil, // ignore ALL system/env proxy configuration
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			c, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, network, addr)
			if err == nil {
				if _, p, e := net.SplitHostPort(c.LocalAddr().String()); e == nil {
					localPort, _ = strconv.Atoi(p)
				}
			}
			return c, err
		},
	}
	client := &http.Client{Transport: tr, Timeout: 3 * time.Second}

	ok := 0
	for i := 0; i < *count; i++ {
		resp, err := client.Get("http://" + *target + "/")
		if err != nil {
			fmt.Fprintln(os.Stderr, "bypassclient: request error:", err)
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		ok++
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("bypassclient pid=%d local_port=%d target=%s ok=%d/%d\n",
		os.Getpid(), localPort, *target, ok, *count)
	fmt.Println("CAP-4 pass criterion: a kernel-scope probe must list local_port above with this PID/image,")
	fmt.Println("even though no proxy was used. A proxy-only capture would miss it entirely.")
}
