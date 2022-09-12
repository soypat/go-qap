package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/soypat/go-qap"
)

//go:embed templates
var templateFS embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error in run: %s", err)
		os.Exit(1)
	}
	log.Println("finished succesfully")
}

var _htmlTemplates *template.Template

func run() error {
	var addr string
	flag.StringVar(&addr, "http", ":8089", "Address on which to serve http.")
	flag.Parse()
	tmpl, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*")
	if err != nil {
		return err
	}
	_htmlTemplates = tmpl
	db, err := OpenBoltQAP("qap.db", tmpl)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	defer db.Close()
	sv := http.NewServeMux()
	sv.HandleFunc("/", db.handleLanding)
	sv.HandleFunc("/qap/search", db.handleSearch)
	sv.HandleFunc("/qap/addDocument", db.handleAddDoc)
	sv.HandleFunc("/qap/createProject", db.handleCreateProject)
	sv.HandleFunc("/qap/toCSV", db.handleToCSV)
	sv.HandleFunc("/qap/importCSV", db.handleImportCSV)
	sv.HandleFunc("/qap/downloadDB", db.handleDownloadDB)
	sv.HandleFunc("/qap/doc/", db.handleGetDocument)
	sv.HandleFunc("/qap/structure", db.handleProjectStructure)
	sv.HandleFunc("/service", db.handleServiceCheck)

	log.Println("Server running http://127.0.0.1" + addr)
	return http.ListenAndServe(addr, sv)
}

func httpErr(w http.ResponseWriter, msg string, err error, code int) {
	if err != nil {
		msg = msg + ": " + err.Error()
	}
	msg = "<h4>" + strconv.Itoa(code) + " - " + msg + "</h4>"
	log.Println(msg)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	_htmlTemplates.Lookup("plain.tmpl").Execute(w, msg)
}

// templating functions.
var funcs = template.FuncMap{
	"intRange": func(start, end int) []int {
		n := end - start + 1
		result := make([]int, n)
		for i := 0; i < n; i++ {
			if start < end {
				result[i] = start + i
			} else {
				result[i] = start - i
			}
		}
		return result
	},
	"safe": func(a interface{}) template.HTML {
		switch v := a.(type) {
		case string:
			return template.HTML(v)
		}
		return "type error"
	},
	"headerURL": headerURL,
	"documentURL": func(d document) string {
		hd, err := d.Header()
		if err != nil {
			return ""
		}
		return "/qap/doc/" + hd.String()
	},
	"debug": func(a any) template.HTML {
		b, err := json.Marshal(a)
		if err != nil {
			b, err = json.Marshal(&a)
		}
		if err != nil {
			return template.HTML(err.Error())
		}
		return template.HTML(b)
	},
	"cat": func(str ...string) string {
		return strings.Join(str, "")
	},
}

func headerURL(hd qap.Header) string {
	return "/qap/doc/" + hd.String()
}
