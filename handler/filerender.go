package handler

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gohxs/httpServe/binAssets"
	blackfriday "github.com/russross/blackfriday/v2"
	"golang.org/x/exp/rand"
)

var tmpl = template.New("")

func init() {
	// Load templates from binAssets
	tmplFiles := []string{
		"tmpl/markdown.tmpl", // should automatic set files
		"tmpl/folder.tmpl",
		"tmpl/wasm.tmpl",
	}
	for _, v := range tmplFiles {
		_, err := tmpl.New(v).Parse(string(binAssets.Data[v]))
		if err != nil {
			log.Fatal("Internal error, loading templates")
		}
	}

}

func renderMarkDown(w http.ResponseWriter, r *http.Request, path string) error {
	fileData, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	opt := blackfriday.WithExtensions(blackfriday.CommonExtensions | blackfriday.HeadingIDs | blackfriday.AutoHeadingIDs)

	outputHTML := blackfriday.Run(fileData, opt)

	err = tmpl.ExecuteTemplate(w, "tmpl/markdown.tmpl", map[string]interface{}{
		"rand":       rand.Int(),
		"css":        flagMdCSS,
		"path":       path,
		"outputHTML": template.HTML(string(outputHTML)),
	})
	return err
}

func renderNotFound(w http.ResponseWriter, r *http.Request, path string) error {
	var err error

	err = tmpl.ExecuteTemplate(w, "tmpl/markdown.tmpl", map[string]interface{}{
		"rand":       rand.Int(),
		"css":        flagMdCSS,
		"path":       path,
		"outputHTML": template.HTML("File not found"),
	})
	return err
}

func renderFolder(w http.ResponseWriter, r *http.Request, path string) error {
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
func renderDotPng(w http.ResponseWriter, r *http.Request, path string) error {
	//log.Println("Executing dot for path", path, path[:len(path)-4])
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	cmd := exec.Command("dot", "-Tpng", absPath)
	cmd.Stdout = w
	return cmd.Run()
}

func handleWasm(w http.ResponseWriter, r *http.Request, path string) error {
	log.Println("Compile wasm, path:", path)

	tf, err := ioutil.TempFile(os.TempDir(), "http-serve")
	if err != nil {
		return err
	}
	tf.Close()

	defer os.Remove(tf.Name())

	// BUILDCOMMAND
	errBuf := new(bytes.Buffer)
	cmd := exec.Command("go", "build", "-o", tf.Name(), "./"+path)
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stderr = errBuf
	if err := cmd.Run(); err != nil {
		fmt.Fprint(w, errBuf.String())
		log.Println("err:", err)
		// Print stderr
		return err
	}

	// Code read
	code, err := ioutil.ReadFile(tf.Name())
	if err != nil {
		log.Println("err:", err)
		return err
	}

	// Load wasm exec
	wasmExecNameBuf := new(bytes.Buffer)
	c := exec.Command("go", "env", "GOROOT")
	c.Stdout = wasmExecNameBuf
	c.Run()
	wasmExecName := strings.TrimSpace(wasmExecNameBuf.String())
	wasmExecName = wasmExecName + "/misc/wasm/wasm_exec.js"

	// Read wasm_exec from system dist
	wasmExec, err := ioutil.ReadFile(wasmExecName)
	if err != nil {
		return err
	}
	return tmpl.ExecuteTemplate(w, "tmpl/wasm.tmpl", map[string]interface{}{
		"wasmexec": template.JS(wasmExec),
		"code":     base64.StdEncoding.EncodeToString(code),
	})
}
