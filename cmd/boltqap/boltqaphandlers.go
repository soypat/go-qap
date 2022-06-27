package main

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/soypat/go-qap"
)

func (q *boltqap) handleCreateProject(rw http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("Code")
	err := q.CreateProject(project)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(rw, "project created")
}

func (q *boltqap) handleAddDoc(rw http.ResponseWriter, r *http.Request) {
	var form newDocForm
	err := bindFormToStruct(&form, r)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	prj, eq, dt := qap.ParseDocumentCodes(form.Code)
	if prj == "" || eq == "" || dt == "" {
		http.Error(rw, "invalid code "+form.Code, http.StatusBadRequest)
		return
	}
	now := time.Now()
	doc := document{
		Project:       prj,
		Equipment:     eq,
		DocType:       dt,
		HumanName:     form.HumanName,
		SubmittedBy:   form.SubmittedBy,
		Location:      form.Location,
		FileExtension: form.FileExtension,
		Created:       now,
		Revised:       now,
	}
	err = q.NewMainDocument(doc)
	if err != nil {
		log.Printf("error creating doc %#v", doc)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(rw, "added %s", strings.Join([]string{prj, eq, dt}, "-"))
}

func (q *boltqap) handleSearch(rw http.ResponseWriter, r *http.Request) {
	query := strings.ToUpper(r.URL.Query().Get("Query"))
	if query == "" || len(query) > 22 {
		http.Error(rw, "invalid query", http.StatusBadRequest)
		return
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("PerPage"))
	if perPage < 10 || perPage > 200 {
		perPage = 40
	}
	data := make([]qap.Header, perPage)
	page, _ := strconv.Atoi(r.URL.Query().Get("Page"))
	log.Printf("querying: %q page %d", query, page)
	n, total := q.filter.HumanQuery(data, query, page)
	if n == 0 && total == 0 {
		http.Error(rw, "query returned no results for "+query, http.StatusBadRequest)
		return
	}
	err := q.tmpl.Lookup("search.tmpl").Execute(rw, struct {
		Page     int
		PerPage  int
		LastPage int
		Headers  []qap.Header
		Query    string
	}{
		PerPage:  perPage,
		Query:    url.QueryEscape(query),
		Page:     page,
		LastPage: total / perPage,
		Headers:  data[:n],
	})
	if err != nil {
		log.Println(err)
	}
}

func (q *boltqap) handleLanding(rw http.ResponseWriter, r *http.Request) {
	const lastEditedDays = 10
	var documents []document
	now := time.Now()
	end := now.AddDate(0, 0, -lastEditedDays)
	q.DoDocumentsRange(now, end, func(d document) error {
		documents = append(documents, d)
		return nil
	})
	q.tmpl.Lookup("landing.tmpl").Execute(rw, struct {
		LastEditedDays int
		Docs           []document
	}{
		LastEditedDays: lastEditedDays,
		Docs:           documents,
	})
}

func (q *boltqap) handleToCSV(rw http.ResponseWriter, r *http.Request) {
	const startCap = 1 << 16
	b := bytes.NewBuffer(make([]byte, 0, startCap))
	w := csv.NewWriter(b)
	w.Write(document{}.recordsHeader())
	q.DoDocuments(func(d document) error {
		w.Write(d.records())
		return nil
	})
	w.Flush()
	rw.Header().Set("Content-Type", "text/csv")
	rw.Header().Set("Content-Disposition", "attachment;filename=\"qapDB.csv\"")
	rw.Header().Set("Content-Length", strconv.Itoa(b.Len()))
	_, err := io.Copy(rw, b)
	if err != nil {
		log.Println("in csv encoding", err)
	}
}

func (q *boltqap) handleImportCSV(rw http.ResponseWriter, r *http.Request) {
	c := csv.NewReader(r.Body)
	expect := document{}.recordsHeader()
	c.ReuseRecord = false
	c.FieldsPerRecord = len(expect)
	header, err := c.Read()
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	for i := range expect {
		if header[i] != expect[i] {
			http.Error(rw, fmt.Sprintf("expected csv header %q, got %q", strings.Join(expect, ","), strings.Join(header, ",")), http.StatusBadRequest)
		}
	}
	var documents []document
	names := make(map[qap.Header]struct{})
	for {
		record, err := c.Read()
		if err != nil {
			break
		}
		doc, err := docFromRecord(record)
		if err != nil {
			break
		}
		hd, _ := doc.Header()
		documents = append(documents, doc)
		names[hd] = struct{}{}
	}
	if err != io.EOF {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	err = q.filter.Do(func(i int, h qap.Header) error {
		if _, present := names[h]; present {
			return errors.New(h.String() + " already exists in database, cannot perform import")
		}
		return nil
	})
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	err = q.ImportDocuments(documents)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
}
