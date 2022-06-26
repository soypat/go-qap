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
	Project     string
	Equipment   string
	DocType     string
	SubmittedBy string
	Number      int
	Attachment  int
	HumanName   string
	Created     time.Time
	Revised     time.Time
	Deleted     bool
}

func (d document) recordsHeader() []string {
	return []string{
		"doc#",
		"submitter",
		"human-name",
		"created",
		"revised",
	}
}

func (d document) records() []string {
	return []string{
		d.String(),
		d.SubmittedBy,
		d.HumanName,
		d.Created.Format(time.RFC3339),
		d.Revised.Format(time.RFC3339),
	}
}

func (d document) key() []byte {
	return boltKey(d.Created)
}

func (d document) String() string {
	hd, _ := d.Header()
	return strings.TrimSuffix(hd.String(), ".00")
}

func docFromValue(b []byte) (d document, err error) {
	err = json.Unmarshal(b, &d)
	return d, err
}

func (d document) CodeQuery() string {
	return strings.Join([]string{d.Project, d.Equipment, d.DocType}, "-")
}

func (d document) Header() (qap.Header, error) {
	return qap.ParseDocumentName(fmt.Sprintf("%s-%s-%s-%d.%02d", d.Project, d.Equipment, d.DocType, d.Number, d.Attachment))
}

func (d document) value() []byte {
	b, err := json.Marshal(d)
	if err != nil {
		panic("unreachable")
	}
	return b
}

type newDocForm struct {
	Code        string
	HumanName   string
	SubmittedBy string
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
