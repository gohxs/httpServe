// Simpliest server
package main

//go:generate genversion -out version.go -package main
//go:generate folder2go -handler assets binAssets

import (
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	if isatty.IsTerminal(os.Stderr.Fd()) {
		prettylog.Global()
	}

	log.Println("V:", Version)

	mux := http.NewServeMux()
	c := chain.New(webu.ChainLogger(prettylog.New("serve")))

	mux.HandleFunc("/.httpServe/_reload/", wsrpc.New(wsrpcClient).ServeHTTP)
	mux.HandleFunc("/.httpServe/", binAssets.AssetHandleFunc)
	// Only logs this
	mux.HandleFunc("/", c.Build(fileServe))

	// Load templates from binAssets
	tmplFiles := []string{
		"tmpl/markdown.tmpl", // should automatic set files
		"tmpl/folder.tmpl",
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
		u, err := url.Parse(toWatch)
		if err != nil {
			return nil, err
		}
		absFile, err := filepath.Abs(u.Path[1:])
		if err != nil {
			return nil, err
		}
		err = watcher.Add(absFile) // remove root '/' prefix
		if err != nil {
			log.Printf("Error watching '%s (%s)' -- %s", toWatch, u.Path, err.Error())
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

func fileServe(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[1:]

	if path == "" {
		path = "." // Cur dir
	}

	if strings.Contains(path, "..") { // ServeFile will normalize path
		http.ServeFile(w, r, path)
	}

	fstat, err := os.Stat(path)
	if err != nil {
		webu.WriteStatus(w, http.StatusNotFound)
		return
	}
	// It is a dir
	if fstat.IsDir() {
		indexFile := filepath.Join(path, "index.html")
		if _, err := os.Stat(indexFile); err == nil {
			http.ServeFile(w, r, indexFile)
		}
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
	if strings.HasSuffix(path, ".dot") && r.URL.Query().Get("f") == "png" {
		err := handleDotPng(w, r, path)
		if err != nil {
			webu.WriteStatus(w, http.StatusInternalServerError, err)
		}
		return
	}
	http.ServeFile(w, r, path)
}

func handleMarkDown(w http.ResponseWriter, r *http.Request, path string) error {
	fileData, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	//Testing
	//fileData = bytes.Replace(fileData, []byte(">"), []byte("&gt;"), -1)

	err = tmpl.ExecuteTemplate(w, "tmpl/markdown.tmpl", map[string]interface{}{
		"path":    path,
		"content": template.HTML(string(fileData)),
	})
	return err
}

func handleFolder(w http.ResponseWriter, r *http.Request, path string) error {
	res, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}
	err = tmpl.ExecuteTemplate(w, "tmpl/folder.tmpl", map[string]interface{}{
		"path":    path,
		"content": res,
	})
	return err
}

// Execute command `dot`
func handleDotPng(w http.ResponseWriter, r *http.Request, path string) error {
	//log.Println("Executing dot for path", path, path[:len(path)-4])
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	cmd := exec.Command("dot", "-Tpng", absPath)
	cmd.Stdout = w
	return cmd.Run()
}
