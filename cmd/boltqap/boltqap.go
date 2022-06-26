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

func boltKey(t time.Time) []byte {
	// RFC3339 format allows for sortable keys. See https://github.com/etcd-io/bbolt#range-scans.
	const timeKeyFormat = "2006-01-02T15:04:05Z"
	return []byte(t.Format(timeKeyFormat))
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

func (q *boltqap) NewMainDocument(doc document) error {
	if doc.SubmittedBy == "" {
		return errors.New("empty submitter")
	} else if doc.HumanName == "" {
		return errors.New("empty human name")
	}
	if time.Since(doc.Created) > 24*time.Hour {
		return errors.New("document created too long ago")
	}
	doc.Number = 1 // Number assigned by system.
	header, err := doc.Header()
	if err != nil {
		return errors.New("document invalidly formatted: " + err.Error())
	}
	var equalCode int
	q.filter.Do(func(i int, h qap.Header) error {
		if qap.HeaderCodesEqual(h, header) {
			equalCode++
		}
		return nil
	})
	doc.Number = 1 + equalCode
	header.Number = int32(doc.Number)
	err = q.filter.AddHeader(header)
	if err != nil {
		return errors.New("unexpected error attempting to add document: " + err.Error())
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
		return err
	}
	return nil
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
	return q.db.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			c := b.Cursor()
			var next func() (k, v []byte)
			var cmp func(a, b []byte) bool
			if incrementing {
				next = c.Next
				cmp = func(a, b []byte) bool { return bytes.Compare(a, b) <= 0 }
			} else {
				cmp = func(a, b []byte) bool { return bytes.Compare(a, b) >= 0 }
				next = c.Prev
			}
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
