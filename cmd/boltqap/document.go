package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/soypat/go-qap"
)

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
	Version       string
	Created       time.Time
	Revised       time.Time
	Deleted       bool
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
		d.Version,
		d.SubmittedBy,
		d.HumanName,
		d.Created.Format(timeKeyFormat),
		d.Revised.Format(timeKeyFormat),
		d.FileExtension,
		d.Location,
	}
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
	d := document{
		Project:       rec.Project(),
		Equipment:     rec.Equipment(),
		DocType:       rec.DocumentType(),
		Number:        int(rec.Number),
		Attachment:    int(rec.AttachmentNumber),
		Version:       record[1],
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

func (d document) Info() (qap.DocInfo, error) {
	hd, err := d.Header()
	if err != nil {
		return qap.DocInfo{}, err
	}
	r, err := d.Revision()
	if err != nil {
		return qap.DocInfo{}, err
	}
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

func (d document) Revision() (qap.Revision, error) {
	return qap.ParseRevision(d.Version)
}

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

func haveConflictingKeys(documents []document) bool {
	keys := make(map[[len(timeKeyFormat)]byte]struct{}, len(documents))
	var key [len(timeKeyFormat)]byte
	for _, doc := range documents {
		n := copy(key[:], doc.key())
		if n != len(timeKeyFormat) {
			panic("unreachable")
		}
		if _, exist := keys[key]; exist {
			return true
		}
		keys[key] = struct{}{}
	}
	return false
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
		default:
			return errors.New("field of kind " + field.Kind().String() + " unsupported")
		}
	}
	return nil
}
