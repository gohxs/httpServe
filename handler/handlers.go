package handler

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"errors"

	"github.com/fsnotify/fsnotify"
	"github.com/gohxs/httpServe/binAssets"
	"github.com/gohxs/prettylog"
	"github.com/gohxs/webu"
	"github.com/gohxs/webu/chain"
	"github.com/gorilla/websocket"
)

var (
	flagMdCSS string
)

func init() {
	flag.StringVar(&flagMdCSS, "md-css", "", "add a css file while rendering markdown")
}

// Render Will select a render and output
func Render() *http.ServeMux {
	c := chain.New(webu.ChainLogger(prettylog.New("file")))
	// File muxer
	mux := http.NewServeMux()
	mux.Handle("/.httpServe/_reload", c.Build(http.HandlerFunc(Watcher)))
	mux.Handle("/.httpServe/", http.StripPrefix("/.httpServe", http.HandlerFunc(binData)))

	mux.HandleFunc("/.httpServe/wasm_exec.js", func(w http.ResponseWriter, r *http.Request) {
		out := new(bytes.Buffer)
		c := exec.Command("go", "env", "GOROOT")
		c.Stdout = out
		c.Run()
		p := strings.TrimSpace(out.String())
		fname := p + "/misc/wasm/wasm_exec.js"
		http.ServeFile(w, r, fname)
	})
	// Only logs this
	mux.Handle("/", c.Build(http.HandlerFunc(fileServe)))

	return mux
}

func binData(w http.ResponseWriter, r *http.Request) {
	urlPath := strings.TrimPrefix(r.URL.String(), "/")
	if urlPath == "" {
		urlPath = "index.html"
	}
	data, ok := binAssets.Data[urlPath]
	if !ok {

		webu.WriteStatus(w, http.StatusNotFound, "Not found")
		return
	}
	w.Header().Set("Content-type", mime.TypeByExtension(filepath.Ext(urlPath)))
	w.Write(data)
}
func fileServe(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[1:]

	if path == "" {
		path = "." // Cur dir
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if strings.Contains(path, "..") { // ServeFile will normalize path
		http.ServeFile(w, r, path)
	}

	fstat, err := os.Stat(path)
	if err != nil {
		err := renderNotFound(w, r, path)
		if err != nil {
			webu.WriteStatus(w, http.StatusInternalServerError, err)
		}
		return
	}
	raw := r.URL.Query().Get("raw")

	// It is a dir
	// Handle dir in another method
	if fstat.IsDir() {
		if raw != "1" {
			// Check for index file
			indexFile := filepath.Join(path, "index.html")
			if _, err := os.Stat(indexFile); err == nil {
				http.ServeFile(w, r, indexFile)
				return
			}
			// Check for main.go file
			mainGo := filepath.Join(path, "main.go")
			if _, err := os.Stat(mainGo); err == nil {
				if err := handleWasm(w, r, path); err != nil {
					webu.WriteStatus(w, http.StatusInternalServerError, err)
				}
				return
			}
		}
		if err := renderFolder(w, r, path); err != nil {
			webu.WriteStatus(w, http.StatusInternalServerError, err)
		}
		return
	}

	if raw == "1" {
		http.ServeFile(w, r, path)
	}

	if filepath.Ext(path) == ".md" {
		err := renderMarkDown(w, r, path)
		if err != nil {
			webu.WriteStatus(w, http.StatusInternalServerError, err)
		}
		return
	}
	if strings.HasSuffix(path, ".dot") && r.URL.Query().Get("f") == "png" {
		err := renderDotPng(w, r, path)
		if err != nil {
			webu.WriteStatus(w, http.StatusInternalServerError, err)
		}
		return
	}
	// default
	http.ServeFile(w, r, path)
}

var upgrader = websocket.Upgrader{}

// Watcher websocket handler
func Watcher(w http.ResponseWriter, r *http.Request) {

	log.Println("Starting watcher")
	// Start watcher
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Web socket error", err)
		return
	}

	watcher, err := fsnotify.NewWatcher() // watcher per socket
	if err != nil {
		return
	}
	wsChan := make(chan int, 1)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Remove != 0 {
					continue
				}
				// Delay a bit because sometimes vim removes the file to format
				<-time.After(200 * time.Millisecond)
				err := c.WriteJSON("reload")
				if err != nil {
					log.Println("Sending msg err:", err)
					return
				}

			case <-wsChan:
				return
			}
		}
	}()

	for {
		err := func() error {
			mt, data, err := c.ReadMessage()
			if err != nil {
				return err
			}
			if mt != websocket.TextMessage {
				return nil
			}

			msg := []string{}
			err = json.Unmarshal(data, &msg)
			if err != nil {
				return err
			}
			/////////////
			// message handling
			/////////
			for _, toWatch := range msg {
				u, err := url.Parse(toWatch)
				if err != nil {
					return err
				}
				absFile, err := filepath.Abs(u.Path[1:])
				if err != nil {
					return err
				}
				// if absFile does not exist, watch parent dir (and do this recursively)
				err = errors.New("none")
				for err != nil && absFile != ".." {
					err = watcher.Add(absFile) // remove root '/' prefix
					absFile = filepath.Dir(absFile)
				}
				if err != nil {
					return fmt.Errorf("Error watching '%s (%s)' -- %s", toWatch, u.Path, err.Error())
				}
			}
			return nil
		}()
		if err != nil {
			log.Println("WATCH Error:", err)
			close(wsChan)
			return
		}

	}

}
