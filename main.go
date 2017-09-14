// Simpliest server
package main

//go:generate genversion -out version.go -package main
//go:generate folder2go -handler assets binAssets

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"dev.hexasoftware.com/stdio/wsrpc"
	"github.com/fsnotify/fsnotify"
	"github.com/gohxs/httpServe/binAssets"
	"github.com/gohxs/prettylog"
	"github.com/gohxs/webu"
	"github.com/gohxs/webu/chain"
	isatty "github.com/mattn/go-isatty"
)

var (
	log  = prettylog.New("httpServe")
	tmpl = template.New("")
)

func main() {
	if !isatty.IsTerminal(os.Stderr.Fd()) {
		prettylog.Style.Enable(false)
	}

	prettylog.Global()

	log.Println("V:", Version)

	mux := http.NewServeMux()
	c := chain.New(webu.ChainLogger(prettylog.New("serve")))

	mux.HandleFunc("/.httpServe/_reload/", wsrpc.New(wsrpcClient).ServeHTTP)
	mux.HandleFunc("/.httpServe/", binAssets.AssetHandleFunc)
	// Only logs this
	mux.HandleFunc("/", c.Build(fileServe))

	// Load templates from binAssets
	tmplFiles := []string{
		"tmpl/MD.tmpl",
		"tmpl/Folder.tmpl",
	}
	for _, v := range tmplFiles {
		_, err := tmpl.New(v).Parse(string(binAssets.Data[v]))
		if err != nil {
			log.Fatal("Internal error, loading templates")
		}
	}

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
		log.Println("Listening with port:", port)

		http.Serve(listener, mux)
	}
	//http.ListenAndServe(":8080", http.FileServer(http.Dir('.')))
}

func wsrpcClient(ctx *wsrpc.ClientCtx) {
	watcher, err := fsnotify.NewWatcher() // watcher per socket
	if err != nil {
		return
	}
	defer watcher.Close()

	ctx.Define("watch", func(params ...interface{}) (interface{}, error) {
		toWatch, ok := params[0].(string)
		if !ok {
			return nil, errors.New("Param invalid")
		}
		absFile, err := filepath.Abs(toWatch[1:])
		if err != nil {
			return nil, err
		}
		err = watcher.Add(absFile) // remove root '/' prefix
		if err != nil {
			log.Println("Error watching", err)
		}
		// Request to watch something?
		return true, nil
	})

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Remove != 0 {
				ctx.Call("reload")
			}
		case <-ctx.Done():
			return
		}
	}

}

func handleMarkDown(w http.ResponseWriter, r *http.Request, path string) error {
	fileData, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	err = tmpl.ExecuteTemplate(w, "tmpl/MD.tmpl", map[string]interface{}{
		"path":    path,
		"content": string(fileData),
	})
	return err
}

func handleFolder(w http.ResponseWriter, r *http.Request, path string) error {
	res, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}
	err = tmpl.ExecuteTemplate(w, "tmpl/Folder.tmpl", map[string]interface{}{
		"path":    path,
		"content": res,
	})
	return err
}

func fileServe(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[1:]
	if path == "" {
		path = "index.html"
		if _, err := os.Stat(path); os.IsNotExist(err) {
			path = "."
		}
	}

	if strings.Contains(path, "..") {
		http.ServeFile(w, r, path)
	}

	fstat, err := os.Stat(path)
	if err != nil {
		webu.WriteStatus(w, http.StatusNotFound)
		return
	}

	if fstat.IsDir() {
		err := handleFolder(w, r, path)
		if err != nil {
			webu.WriteStatus(w, http.StatusInternalServerError, err)
		}
		return
	}

	if filepath.Ext(path) == ".md" {
		err := handleMarkDown(w, r, path)
		if err != nil {
			webu.WriteStatus(w, http.StatusInternalServerError, err)
		}
		return
	}
	http.ServeFile(w, r, path)
}
