package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/soypat/go-qap"
)

type revision struct {
	Index       qap.Revision
	Description string
}

type document struct {
	Project       string
	Equipment     string
	DocType       string
	SubmittedBy   string
	Number        int
	Attachment    int
	HumanName     string
	FileExtension string
	Location      string
	Created       time.Time
	Revised       time.Time
	Deleted       bool
	// Revisions is stored DB side only.
	Revisions   []revision
	Attachments []qap.Header
}

func (doc document) ValidateForAdmission() (qap.DocInfo, error) {
	info, err := doc.Info()
	if err != nil {
		return info, err
	}
	switch {
	case doc.SubmittedBy == "":
		return info, errors.New("empty submitter")
	case doc.HumanName == "":
		return info, errors.New("empty human name")
	case doc.FileExtension == "":
		return info, errors.New("empty file extension")
	case doc.Location == "":
		return info, errors.New("empty location")
	case time.Since(doc.Created) > 24*time.Hour:
		return info, errors.New("document created too long ago")
	}
	return info, nil
}

func (d document) recordsHeader() []string {
	return []string{
		"doc#",
		"version",
		"submitter",
		"human-name",
		"created",
		"revised",
		"file-ext",
		"location",
	}
}

func (d document) records() []string {
	return []string{
		d.String(),
		d.Version(),
		d.SubmittedBy,
		d.HumanName,
		d.Created.Format(timeKeyFormat),
		d.Revised.Format(timeKeyFormat),
		d.FileExtension,
		d.Location,
	}
}

func (d document) Version() string {
	if len(d.Revisions) == 0 {
		return qap.NewRevision().String()
	}
	return d.Revisions[len(d.Revisions)-1].Index.String()
}

func docFromRecord(record []string, ignoreTime bool) (document, error) {
	if len(record) < len(document{}.recordsHeader()) {
		return document{}, errors.New("not enough record fields to parse document")
	}
	rec, err := qap.ParseHeader(record[0], false)
	if err != nil {
		rec, err = qap.ParseHeader(record[0], true)
	}
	if err != nil {
		return document{}, errors.New("parsing document name" + record[0] + " from record: " + err.Error())
	}
	var created, revised time.Time
	if !ignoreTime {
		created, err = time.Parse(timeKeyFormat, record[4])
		if err != nil {
			return document{}, errors.New("parsing doc record creation field: " + err.Error())
		}
		revised, err = time.Parse(timeKeyFormat, record[5])
		if err != nil {
			return document{}, errors.New("parsing doc record creation field: " + err.Error())
		}
	}
	rev, err := qap.ParseRevision(record[1])
	if err != nil {
		return document{}, err
	}
	d := document{
		Project:       rec.Project(),
		Equipment:     rec.Equipment(),
		DocType:       rec.DocumentType(),
		Number:        int(rec.Number),
		Attachment:    int(rec.AttachmentNumber),
		Revisions:     []revision{{Index: rev}},
		SubmittedBy:   record[2],
		HumanName:     record[3],
		Created:       created,
		Revised:       revised,
		FileExtension: record[6],
		Location:      record[7],
	}
	_, err = d.Info()
	if err != nil && !(errors.Is(err, qap.ErrZeroTime) && ignoreTime) {
		return document{}, err
	}
	return d, nil
}

func (d document) key() []byte {
	return boltKey(d.Created)
}

func (d *document) AddRevision(rev revision) error {
	for i := range d.Revisions {
		if d.Revisions[i].Index == rev.Index {
			return errors.New("document revision index already exists")
		}
	}
	d.Revisions = append(d.Revisions, rev)
	return nil
}

func (d document) Info() (qap.DocInfo, error) {
	hd, err := d.Header()
	if err != nil {
		return qap.DocInfo{}, err
	}
	r := d.Revision()
	di := qap.DocInfo{
		Header:       hd,
		Revision:     r,
		Creation:     d.Created,
		RevisionTime: d.Revised,
	}
	if err := di.Validate(); err != nil {
		return qap.DocInfo{}, err
	}
	return di, nil
}

func (d document) Revision() qap.Revision {
	if len(d.Revisions) == 0 {
		return qap.NewRevision()
	}
	return d.Revisions[len(d.Revisions)-1].Index
}

// String returns the Header's document name representation i.e. "SPS-PEC-HP-023
func (d document) String() string {
	di, err := d.Info()
	if err != nil {
		return "<invalid document>"
	}
	return strings.TrimSuffix(di.Header.String(), ".00")
}

func docFromValue(b []byte) (d document, err error) {
	err = json.Unmarshal(b, &d)
	return d, err
}

func (d document) URL() string {
	hd, err := d.Header()
	if err != nil {
		return "/qap/doc/invalid"
	}
	return "/qap/doc/" + hd.String()
}

func (d document) CodeQuery() string {
	return strings.Join([]string{d.Project, d.Equipment, d.DocType}, "-")
}

func (d document) Header() (qap.Header, error) {
	return qap.ParseHeader(fmt.Sprintf("%s-%s-%s-%d.%02d", d.Project, d.Equipment, d.DocType, d.Number, d.Attachment), false)
}

func (d document) value() []byte {
	b, err := json.Marshal(d)
	if err != nil {
		panic("unreachable")
	}
	return b
}

func consolidateMainDocumentVersions(documents []document) ([]document, error) {
	mdoc := make(map[qap.Header]document)
	for _, doc := range documents {
		hd, err := doc.Header()
		if err != nil {
			return nil, err
		}
		rev := doc.Revision()
		got, ok := mdoc[hd]
		if !ok {
			doc.AddRevision(revision{Index: rev})
			mdoc[hd] = doc
			continue
		}
		// we have two documents of identical header
		if got.Revision() == doc.Revision() {
			return nil, fmt.Errorf("conflicting document %s rev %s", doc.String(), doc.Revision())
		}
		err = doc.AddRevision(revision{Index: rev})
		if err != nil {
			return nil, fmt.Errorf("attempting to merge document %s revision: %s", doc.String(), err)
		}
	}
	var newDocs []document
	for _, d := range mdoc {
		newDocs = append(newDocs, d)
	}
	return newDocs, nil
}

func checkConflicts(documents []document) error {
	names := make(map[qap.Header]struct{})
	keys := make(map[[len(timeKeyFormat)]byte]struct{}, len(documents))
	var key [len(timeKeyFormat)]byte
	for _, doc := range documents {
		n := copy(key[:], doc.key())
		if n != len(timeKeyFormat) {
			return errors.New("unexpected error in database key format")
		}
		if _, exist := keys[key]; exist {
			return errors.New("conflicting key " + string(key[:]))
		}
		hd, err := doc.Header()
		if err != nil {
			return err
		}
		if _, exist := names[hd]; exist {
			return fmt.Errorf("conflicting header %s", hd.String())
		}
		keys[key] = struct{}{}
		names[hd] = struct{}{}
	}
	return nil
}

type newDocForm struct {
	Code          string
	HumanName     string
	SubmittedBy   string
	FileExtension string
	Location      string
}

func bindFormToStruct(a any, r *http.Request) error {
	rv := reflect.ValueOf(a)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("argument must be pointer and non nil type")
	}
	rv = reflect.Indirect(rv)
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		name := rv.Type().Field(i).Name
		if !r.URL.Query().Has(name) {
			return errors.New(name + " query value not found")
		}
		val := r.URL.Query().Get(name)
		switch field.Kind() {
		case reflect.String:
			field.Set(reflect.ValueOf(val))
		case reflect.Int:
			d, err := strconv.Atoi(val)
			if err != nil {
				return errors.New("could not parse integer: " + err.Error())
			}
			field.Set(reflect.ValueOf(d))
		default:
			return errors.New("field of kind " + field.Kind().String() + " unsupported")
		}
	}
	return nil
}

func createDocumentFromForm(r *http.Request) (document, error) {
	var form newDocForm
	err := bindFormToStruct(&form, r)
	if err != nil {
		return document{}, err
	}
	prj, eq, dt := qap.ParseDocumentCodes(form.Code)
	if prj == "" || eq == "" || dt == "" {
		return document{}, err
	}
	now := time.Now()
	return document{
		Project:       prj,
		Equipment:     eq,
		DocType:       dt,
		HumanName:     form.HumanName,
		SubmittedBy:   form.SubmittedBy,
		Location:      form.Location,
		FileExtension: form.FileExtension,
		Created:       now,
		Revised:       now,
	}, nil
}
