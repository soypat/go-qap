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

func (q *boltqap) handleDownloadDB(rw http.ResponseWriter, r *http.Request) {
	tx, err := q.db.Begin(false)
	if err != nil {
		httpErr(rw, "could not begin transaction to write DB", err, http.StatusInternalServerError)
		return
	}
	var b bytes.Buffer
	n, err := tx.WriteTo(&b)
	if err != nil {
		httpErr(rw, "during DB buffer out", err, http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "applications/octet-stream")
	rw.Header().Set("Content-Disposition", "attachment;filename=\"qap.bbolt\"")
	rw.Header().Set("Content-Length", strconv.FormatInt(n, 10))
	_, err = io.Copy(rw, &b)
	if err != nil {
		httpErr(rw, "during DB write out to network", err, http.StatusInternalServerError)
		return
	}
}

func (q *boltqap) handleGetDocument(rw http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path[1:]
	fpath := path.Base(upath)
	hd, err := qap.ParseHeader(fpath, false)
	if err != nil {
		hd, err = qap.ParseHeader(fpath, true)
	}
	if err != nil {
		httpErr(rw, "parsing document header", err, http.StatusBadRequest)
		return
	}
	log.Println("get document", hd.String())
	doc, err := q.FindDocument(hd)
	if err != nil {
		httpErr(rw, "error looking for document", err, http.StatusInternalServerError)
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
	query := r.URL.Query()
	project := query.Get("newcode")
	name := query.Get("name")
	desc := query.Get("desc")
	err := q.CreateProject(project, name, desc)
	if err != nil {
		httpErr(rw, "creating project", err, http.StatusInternalServerError)
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
		httpErr(rw, "creating doc "+doc.String(), err, http.StatusInternalServerError)
		return
	}
	log.Printf("added %s", doc.String())
	http.Redirect(rw, r, newdoc.URL(), http.StatusTemporaryRedirect)
}

func (q *boltqap) handleSearch(rw http.ResponseWriter, r *http.Request) {
	hq := r.URL.Query()
	query := strings.ToUpper(hq.Get("Query"))
	if query == "" || len(query) > 22 {
		httpErr(rw, "invalid query", nil, http.StatusBadRequest)
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
		httpErr(rw, "query returned no results for "+query, nil, http.StatusBadRequest)
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
	if r.URL.Path != "/" {
		log.Println("unknown path ", r.URL.Path)
		httpErr(rw, "404 page not found", nil, 404)
		return
	}
	log.Println("landing")
	const lastEditedDays = 10
	var documents []document
	now := time.Now()
	end := now.AddDate(0, 0, -lastEditedDays)
	var projects []qap.Project
	q.DoDocumentsRange(now, end, func(d document) error {
		documents = append(documents, d)
		return nil
	})
	q.DoProjects(func(structure qap.Project) error {
		projects = append(projects, structure)
		return nil
	})
	rw.WriteHeader(200)
	q.tmpl.Lookup("landing.tmpl").Execute(rw, struct {
		LastEditedDays int
		Docs           []document
		Projects       []qap.Project
	}{
		LastEditedDays: lastEditedDays,
		Docs:           documents,
		Projects:       projects,
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
		httpErr(rw, "opening multipart form file", err, http.StatusInternalServerError)
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
		isRelease := query.Get("isrelease") == "on"
		revStr := query.Get("rev")
		description := query.Get("desc")
		rev, err := qap.ParseRevision(revStr)
		if err != nil {
			httpErr(rw, "parsing revision \""+revStr+"\"", err, http.StatusBadRequest)
			return
		}
		if len(description) == 0 {
			httpErr(rw, "empty description", nil, http.StatusBadRequest)
			return
		}
		if isRelease && !rev.IsRelease {
			httpErr(rw, "IsRelease form mismatch", nil, http.StatusBadRequest)
			return
		}
		rev.IsRelease = isRelease // override to draft status unless specified otherwise.
		err = b.AddRevision(hd, revision{
			Index:       rev,
			Description: description,
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
		return
	}
	log.Printf("document action %s success", action)
	http.Redirect(rw, r, headerURL(hd), http.StatusTemporaryRedirect)
}

func (q *boltqap) handleProjectStructure(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	project := query.Get("project")
	if len(project) < 3 {
		httpErr(rw, "project name too short", nil, http.StatusBadRequest)
		return
	}
	structure, err := q.GetStructure(project[:3])
	if err != nil {
		httpErr(rw, "while looking for project structure", err, http.StatusInternalServerError)
		return
	}
	code := query.Get("newcode")
	if code != "" {
		q.handleAddEquipmentCode(rw, r, structure)
		return
	}
	err = q.tmpl.Lookup("project.tmpl").Execute(rw, structure)
	if err != nil {
		httpErr(rw, "template exec", err, http.StatusInternalServerError)
		return
	}
}

func (q *boltqap) handleAddEquipmentCode(rw http.ResponseWriter, r *http.Request, structure qap.Project) {
	query := r.URL.Query()
	code := query.Get("newcode")
	name := reName.FindString(query.Get("name"))
	desc := query.Get("desc")
	if name == "" || desc == "" {
		httpErr(rw, "invalid name or empty description", nil, http.StatusBadRequest)
		return
	}
	accum := query.Get("accum")
	err := structure.AddEquipmentCode(accum+code, name, desc)
	if err != nil {
		httpErr(rw, "adding equipment code", err, http.StatusInternalServerError)
		return
	}
	err = q.PutStructure(structure)
	if err != nil {
		httpErr(rw, "modifying project structure in DB", err, http.StatusInternalServerError)
		return
	}
	log.Println("added ", accum+code, " to structure: ", structure)
	http.Redirect(rw, r, "/qap/structure?project="+structure.Project(), http.StatusTemporaryRedirect)
}
