package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"log"
	"strings"
	"time"

	"github.com/soypat/go-qap"
	"go.etcd.io/bbolt"
)

const timeKeyFormat = "2006-01-02 15:04:05.9999"

func boltKey(t time.Time) []byte {
	padding := [5]byte{'.', '0', '0', '0', '0'}
	// RFC3339 format allows for sortable keys. See https://github.com/etcd-io/bbolt#range-scans.
	key := []byte(t.Format(timeKeyFormat))
	diff := len(timeKeyFormat) - len(key)
	key = append(key, padding[len(padding)-diff:]...)
	return key
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

type boltqap struct {
	db     *bbolt.DB
	filter qap.HeaderFilter
	tmpl   *template.Template
}

func (q *boltqap) CreateProject(projectName string) error {
	if len(projectName) != 3 || strings.ToUpper(projectName) != projectName {
		return errors.New("project name must be of length 3 and all upper case")
	}
	projectName, _, _ = qap.ParseDocumentCodes(projectName)
	if projectName == "" {
		return errors.New("invalid project name: " + qap.ErrBadProjectCode.Error())
	}
	err := q.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucket([]byte(projectName))
		return err
	})
	if err != nil {
		return errors.New("error creating project, probably already exists: " + err.Error())
	}
	return nil
}

func (q *boltqap) NewMainDocument(doc document) (newdoc document, err error) {
	if doc.Version == "" {
		doc.Version = qap.NewRevision().String()
	}
	switch {
	case doc.SubmittedBy == "":
		return document{}, errors.New("empty submitter")
	case doc.HumanName == "":
		return document{}, errors.New("empty human name")
	case doc.FileExtension == "":
		return document{}, errors.New("empty file extension")
	case doc.Location == "":
		return document{}, errors.New("empty location")
	case time.Since(doc.Created) > 24*time.Hour:
		return document{}, errors.New("document created too long ago")
	}
	doc.Revised = time.Now()
	doc.Number = 1 // Actual number assigned below.
	header, err := doc.Header()
	if err != nil {
		return document{}, errors.New("document header invalidly formatted: " + err.Error())
	}
	_, err = doc.Revision()
	if err != nil {
		return document{}, errors.New("document revision invalidly formatted: " + err.Error())
	}
	var maxCode int32
	q.filter.Do(func(i int, h qap.Header) error {
		if qap.HeaderCodesEqual(h, header) {
			if h.Number > maxCode {
				maxCode = h.Number
			}
		}
		return nil
	})
	doc.Number = int(maxCode) + 1
	header.Number = maxCode + 1
	err = q.filter.AddHeader(header)
	if err != nil {
		return newdoc, errors.New("unexpected error attempting to add document: " + err.Error())
	}
	err = q.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(header.Project()))
		if b == nil {
			return errors.New("project not exist")
		}
		key := doc.key()
		v := b.Get(key)
		if v != nil {
			return errors.New("key already exists in document")
		}
		err = b.Put(key, doc.value())
		if err != nil {
			return fmt.Errorf("while putting document %v in database: %s", doc, err)
		}
		return nil
	})
	if err != nil {
		return document{}, err
	}
	return doc, nil
}

func (q *boltqap) DoDocuments(f func(d document) error) error {
	return q.db.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			return b.ForEach(func(k, v []byte) error {
				doc, err := docFromValue(v)
				if err != nil {
					log.Println("error reading document from database: ", err.Error())
					return nil
				}
				return f(doc)
			})
		})
	})
}

func (q *boltqap) DoDocumentsRange(startTime, endTime time.Time, f func(d document) error) error {
	incrementing := startTime.Before(endTime)
	start := boltKey(startTime)
	end := boltKey(endTime)
	var getIterator func(*bbolt.Cursor) func() (k, v []byte)
	var cmp func(a, b []byte) bool
	if incrementing {
		getIterator = func(c *bbolt.Cursor) func() (k, v []byte) { return c.Next }
		cmp = func(a, b []byte) bool { return bytes.Compare(a, b) <= 0 }
	} else {
		getIterator = func(c *bbolt.Cursor) func() (k, v []byte) { return c.Prev }
		cmp = func(a, b []byte) bool { return bytes.Compare(a, b) >= 0 }
	}
	return q.db.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			c := b.Cursor()
			next := getIterator(c)
			k, v := c.Seek(start)
			if k == nil {
				k, v = next()
			}
			for ; k != nil && cmp(k, end); k, v = next() {
				d, err := docFromValue(v)
				if err != nil {
					log.Println("error reading document:" + err.Error())
					continue
				}
				err = f(d)
				if err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func (q *boltqap) ImportDocuments(documents []document) (err error) {
	if err := checkConflicts(documents); err != nil {
		return errors.New("error checking document conflicts during import: " + err.Error())
	}
	for _, doc := range documents {
		info, err := doc.Info()
		if err != nil {
			return err
		}
		if q.filter.Has(info.Header) {
			return fmt.Errorf("%s already exists", info.Header)
		}
	}
	tx, err := q.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	err = q.importDocuments(tx, documents)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, doc := range documents {
		// Documents are guaranteed to be valid by this point.
		hd, _ := doc.Header()
		q.filter.AddHeader(hd)
	}
	return nil
}

func (q *boltqap) importDocuments(tx *bbolt.Tx, documents []document) error {
	for _, doc := range documents {
		bucket := tx.Bucket([]byte(doc.Project))
		if bucket == nil {
			return errors.New(doc.Project + "project not found")
		}
		key := doc.key()
		existing := bucket.Get(key)
		if existing != nil {
			return fmt.Errorf("imported document %q cannot have same creation time as existing document", doc.String())
		}
		err := bucket.Put(key, doc.value())
		if err != nil {
			return err
		}
	}
	return nil
}

func (q *boltqap) GetDocument(hd qap.Header) (document, error) {
	thedoc := document{}
	gotDoc := errors.New("got the document")
	err := q.db.View(func(tx *bbolt.Tx) error {
		buck := tx.Bucket([]byte(hd.Project()))
		if buck == nil {
			return errors.New("project not exist:" + hd.Project())
		}
		return buck.ForEach(func(k, v []byte) error {
			doc, err := docFromValue(v)
			if err != nil {
				log.Println("error reading document from database: ", err.Error())
				return nil
			}
			hdgot, err := doc.Header()
			if qap.HeadersEqual(hd, hdgot) {
				thedoc = doc
				return gotDoc
			}
			return nil
		})
	})
	if err == nil {
		return thedoc, errors.New("did not find document" + hd.String())
	}
	if !errors.Is(err, gotDoc) {
		return thedoc, err
	}
	return thedoc, nil
}
