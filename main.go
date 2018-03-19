// Simpliest server
package main

//go:generate genversion -out version.go -package main
//go:generate folder2go -nobackup -handler assets binAssets

import (
	"bytes"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/gohxs/httpServe/handler"
	"github.com/gohxs/prettylog"
	"github.com/gohxs/webu"
	"github.com/gohxs/webu/chain"
	isatty "github.com/mattn/go-isatty"
)

var (
	log = prettylog.New("httpServe")

	// Flags
	proxyTo string
)

func main() {
	if isatty.IsTerminal(os.Stderr.Fd()) {
		prettylog.Global()
	}

	flag.StringVar(&proxyTo, "proxy", "", "do not serve files only creates a reverse proxy")
	flag.Parse()
	log.Println("V:", Version)

	var r http.Handler
	if len(proxyTo) != 0 {
		c := chain.New(webu.ChainLogger(prettylog.New("proxy")))
		log.Println("Proxy to:", proxyTo)
		u, err := url.Parse(proxyTo)
		if err != nil {
			log.Fatal(err)
		}
		r = c.Build(httputil.NewSingleHostReverseProxy(u).ServeHTTP)
	} else {
		r = handler.Render()
	}

	// Initial port
	var port = 8080

	for {
		addr := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Println("Err opening", port, err)
			port++
			log.Println("Trying port", port)
			continue
		}

		log.Printf("Listening at:")

		addrW := bytes.NewBuffer(nil)
		fmt.Fprintf(addrW, "    http://localhost:%d\n", port)
		addrs, err := net.InterfaceAddrs()
		for _, a := range addrs {
			astr := a.String()
			if strings.HasPrefix(astr, "192.168") ||
				strings.HasPrefix(astr, "10") {
				a := strings.Split(astr, "/")[0]
				fmt.Fprintf(addrW, "    http://%s:%d\n", a, port)
			}
		}
		log.Println(addrW.String())

		http.Serve(listener, r)
	}
	//http.ListenAndServe(":8080", http.FileServer(http.Dir('.')))
}
