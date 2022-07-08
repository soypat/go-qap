package main

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/soypat/go-qap"
)

func (q *boltqap) handleGetDocument(rw http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path[1:]
	fpath := path.Base(upath)
	hd, err := qap.ParseHeader(fpath, true)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	log.Println("get document", hd.String())
	doc, err := q.FindMainDocument(hd)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	query := r.URL.Query()
	if query.Has("action") {
		q.handleDocumentAction(rw, r, doc, query)
		return
	}
	err = q.tmpl.Lookup("document.tmpl").Execute(rw, doc)
	if err != nil {
		log.Println("error in document template: ", err)
	}
}

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
	doc, err := createDocumentFromForm(r)
	if err != nil {
		httpErr(rw, "could not create document from form", err, http.StatusBadRequest)
		return
	}
	newdoc, err := q.NewMainDocument(doc)
	if err != nil {
		log.Printf("error creating doc %#v", doc)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("added %s", doc.String())
	http.Redirect(rw, r, "/qap/doc/"+newdoc.String(), http.StatusTemporaryRedirect)
}

func (q *boltqap) handleSearch(rw http.ResponseWriter, r *http.Request) {
	hq := r.URL.Query()
	query := strings.ToUpper(hq.Get("Query"))
	if query == "" || len(query) > 22 {
		http.Error(rw, "invalid query", http.StatusBadRequest)
		return
	}
	perPage, _ := strconv.Atoi(hq.Get("PerPage"))
	if perPage < 10 || perPage > 200 {
		perPage = 40
	}
	data := make([]qap.Header, perPage)
	page, _ := strconv.Atoi(hq.Get("Page"))
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
	log.Println("landing")
	const lastEditedDays = 10
	var documents []document
	now := time.Now()
	end := now.AddDate(0, 0, -lastEditedDays)
	q.DoDocumentsRange(now, end, func(d document) error {
		documents = append(documents, d)
		return nil
	})
	rw.WriteHeader(200)
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
	const megabyte = 1000 * 1000
	err := r.ParseMultipartForm(12 * megabyte)
	if err != nil {
		httpErr(rw, err.Error(), nil, http.StatusInternalServerError)
		return
	}
	files := r.MultipartForm.File["ImportCSV"]
	if len(files) != 1 {
		httpErr(rw, "ImportCSV file not found or too many files", nil, http.StatusBadRequest)
		return
	}
	f, err := files[0].Open()
	if err != nil {
		httpErr(rw, err.Error(), nil, http.StatusInternalServerError)
		return
	}
	c := csv.NewReader(f)
	expect := document{}.recordsHeader()
	c.ReuseRecord = false
	c.FieldsPerRecord = len(expect)
	header, err := c.Read()
	if err != nil {
		httpErr(rw, "parsing csv header", err, http.StatusBadRequest)
		return
	}
	for i := range expect {
		if header[i] != expect[i] {
			httpErr(rw, fmt.Sprintf("expected csv header %q, got %q", strings.Join(expect, ","), strings.Join(header, ",")), nil, http.StatusBadRequest)
			return
		}
	}
	var documents []document
	var record []string
	var err1 error
	now := time.Now()
	for {
		record, err1 = c.Read()
		if err1 != nil {
			err = err1
			break
		}
		doc, err1 := docFromRecord(record, false)
		if err1 != nil {
			doc, err1 = docFromRecord(record, true)
			doc.Created = now
			doc.Revised = now
			now = now.Add(time.Millisecond)
		}
		if err1 != nil {
			err = err1
			break
		}
		documents = append(documents, doc)
	}
	if err != io.EOF && err != nil {
		httpErr(rw, fmt.Sprintf("parsing csv data %v", record), err, http.StatusBadRequest)
		return
	}
	documents, err = consolidateMainDocumentVersions(documents)
	if err != nil {
		httpErr(rw, "consolidating main doc versions", err, http.StatusBadRequest)
		return
	}
	err = q.ImportDocuments(documents)
	if err != nil {
		httpErr(rw, "importing doc", err, http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(rw, "success writing %d documents to database", len(documents))
}

func (b *boltqap) handleDocumentAction(rw http.ResponseWriter, r *http.Request, doc document, query url.Values) {
	hd, _ := doc.Header()
	action := query.Get("action")
	switch action {
	case "addRevision":
		revStr := query.Get("rev")
		rev, err := qap.ParseRevision(revStr)
		if err != nil {
			httpErr(rw, "parsing revision \""+revStr+"\"", err, http.StatusBadRequest)
			return
		}
		err = b.AddRevision(hd, revision{
			Index:       rev,
			Description: query.Get("desc"),
		})
		if err != nil {
			httpErr(rw, "adding revision", err, http.StatusInternalServerError)
			return
		}
	case "addAttachment":
		attachment, err := createDocumentFromForm(r)
		if err != nil {
			httpErr(rw, "creating document from form", err, http.StatusBadRequest)
			return
		}

		newAttachment := uint8(1)
		for i := range doc.Attachments {
			if doc.Attachments[i].AttachmentNumber >= newAttachment {
				newAttachment = doc.Attachments[i].AttachmentNumber + 1
			}
		}
		attachment.Attachment = int(newAttachment)
		attachment.Number = doc.Number
		ainfo, err := attachment.Info()
		if err != nil {
			httpErr(rw, "attachment malformed", err, http.StatusInternalServerError)
			return
		}
		err = b.NewDocument(attachment)
		if err != nil {
			httpErr(rw, "adding attachment to DB ", err, http.StatusInternalServerError)
			return
		}
		doc.Attachments = append(doc.Attachments, ainfo.Header)
		err = b.Update(doc)
		if err != nil {
			httpErr(rw, "updating existing document", err, http.StatusInternalServerError)
			return
		}

	default:
		httpErr(rw, "action not found: "+action, nil, http.StatusBadRequest)
	}
	log.Printf("document action %s success", action)
	http.Redirect(rw, r, headerURL(hd), http.StatusTemporaryRedirect)
}
