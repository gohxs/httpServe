// Simpliest server
package main

//go:generate folder2go assets binAssets

import (
	"fmt"
	"hexasoftware/cmd/httpServe/binAssets"
	_ "hexasoftware/lib/prettylog/global"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func CreateHandleFunc(prefix string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var solvedPath = r.URL.Path
		if solvedPath == prefix {
			solvedPath = prefix + "/index.html"
		}
		log.Printf("%s - (embed)%s", r.Method, r.URL.Path)
		if strings.HasPrefix(solvedPath, prefix) {
			solvedPath = solvedPath[len(prefix):]
		}
		data, ok := binAssets.Data[solvedPath]
		if !ok {
			w.WriteHeader(404)
		}
		w.Header().Set("Content-type", mime.TypeByExtension(filepath.Ext(solvedPath)))
		w.Write(data)
	}
}

func HandleMarkDown(w http.ResponseWriter, r *http.Request, path string) error {
	log.Println("Handling markdown")
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	/* // Server side markdown2html
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	w.Write([]byte("<html><head><link rel='stylesheet' href='/.httpServe/strapdown.css'><link rel='stylesheet' href='/.httpServe/themes/united.min.css'></head><body>"))
	mdData := blackfriday.MarkdownCommon(data)
	w.Write(mdData)
	w.Write([]byte("</body></html>"))
	return nil
	*/
	w.Write([]byte(
		`
<!DOCTYPE html>
<html>
<xmp theme="paper" style="display:none;">
`))

	io.Copy(w, f)
	w.Write([]byte(
		`
			</xmp>
	<script src=".httpServe/strapdown.js"></script>
</html>`))
	return nil
}

func HandleFolder(w http.ResponseWriter, r *http.Request, path string) error {

	res, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}
	w.Write([]byte(
		`<html>
 <body>
 <ul>`))
	for _, f := range res {
		w.Write([]byte(fmt.Sprintf(`<li><a href="/%s">%s</a>`, path+"/"+f.Name(), f.Name())))
	}
	w.Write([]byte(
		`</ul>
 </body>
 </html>
 `))

	return nil
}

type FileServer struct {
}

func (FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	log.Printf("%s - %s", r.Method, r.URL.Path)

	fstat, err := os.Stat(path)
	if err != nil {
		log.Println("ERR:", err)
		http.ServeFile(w, r, path)
		return
	}
	if fstat.IsDir() {
		HandleFolder(w, r, path)
		return
	}

	if filepath.Ext(path) == ".md" {
		err := HandleMarkDown(w, r, path)
		if err != nil {
			http.ServeFile(w, r, path)
		}
		return
	}

	// Default file server
	http.ServeFile(w, r, path)
}

func main() {

	mux := http.NewServeMux()
	mux.HandleFunc("/.httpServe/", CreateHandleFunc("/.httpServe"))
	mux.Handle("/", FileServer{})
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
