package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/soypat/go-qap"
	"go.etcd.io/bbolt"
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

func run() error {
	var addr string
	flag.StringVar(&addr, "http", ":8089", "Address on which to serve http.")
	flag.Parse()
	bolt, err := bbolt.Open("qap.db", 0666, nil)
	if err != nil {
		return err
	}
	defer bolt.Close()
	headers := make([]qap.Header, 0, 1024)
	err = bolt.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			log.Printf("found project %s with %d keys", name, b.Stats().KeyN)
			return b.ForEach(func(_, v []byte) error {
				doc, err := docFromValue(v)
				if err != nil {
					return err
				}
				if doc.Deleted {
					return nil
				}
				hd, err := doc.Header()
				if err != nil {
					return err
				}
				headers = append(headers, hd)
				return nil
			})

		})
	})
	if err != nil {
		return fmt.Errorf("initializing headers from file data: %s", err)
	}
	tmpl, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*")
	if err != nil {
		return err
	}
	db := &boltqap{
		db:     bolt,
		filter: qap.NewHeaderFilter(headers),
		tmpl:   tmpl,
	}

	sv := http.NewServeMux()
	sv.HandleFunc("/", db.handleLanding)
	sv.HandleFunc("/qap/search", db.handleSearch)
	sv.HandleFunc("/qap/addDocument", db.handleAddDoc)
	sv.HandleFunc("/qap/createProject", db.handleCreateProject)
	sv.HandleFunc("/qap/toCSV", db.handleToCSV)
	sv.HandleFunc("/qap/importCSV", db.handleImportCSV)
	sv.HandleFunc("/qap/doc/", db.handleGetDocument)
	log.Println("Server running http://127.0.0.1" + addr)
	return http.ListenAndServe(addr, sv)
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
	"headerURL": func(hd qap.Header) string {
		return "/qap/doc/" + hd.String()
	},
}
