package handler

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"

	"github.com/gohxs/httpServe/binAssets"
	"golang.org/x/exp/rand"
	blackfriday "gopkg.in/russross/blackfriday.v2"
)

var tmpl = template.New("")

func init() {
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
