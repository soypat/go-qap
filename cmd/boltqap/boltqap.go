package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/soypat/go-qap"
	"go.etcd.io/bbolt"
)

// ErrEndLookup ends document lookup functions gracefully.
var ErrEndLookup = errors.New("lookup ended")

const timeKeyFormat = "2006-01-02 15:04:05.9999"

func boltKey(t time.Time) []byte {
	padding := [5]byte{'.', '0', '0', '0', '0'}
	// RFC3339 format allows for sortable keys. See https://github.com/etcd-io/bbolt#range-scans.
	key := []byte(t.Format(timeKeyFormat))
	diff := len(timeKeyFormat) - len(key)
	key = append(key, padding[len(padding)-diff:]...)
	return key
}

func OpenBoltQAP(dbname string, templates *template.Template) (*boltqap, error) {
	bolt, err := bbolt.Open(dbname, 0666, nil)
	if err != nil {
		return nil, err
	}

	q := &boltqap{
		db:   bolt,
		tmpl: templates,
	}
	headers := make([]qap.Header, 0, 1024)
	err = q.DoDocuments(func(doc document) error {
		hd, err := doc.Header()
		if err != nil {
			return err
		}
		headers = append(headers, hd)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("initializing headers from file data: %s", err)
	}
	q.filter = qap.NewHeaderFilter(headers)
	return q, nil
}

func (q *boltqap) Close() error { return q.db.Close() }

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

type boltqap struct {
	db       *bbolt.DB
	filter   qap.HeaderFilter
	tmpl     *template.Template
	projects map[string]qap.Project
}

var reName = regexp.MustCompile(`^[a-zA-Z+-]+$`)

func (q *boltqap) CreateProject(code, name, desc string) error {
	if len(code) != 3 || strings.ToUpper(code) != code {
		return errors.New("project name must be of length 3 and all upper case")
	}
	name = reName.FindString(name)
	if name == "" || desc == "" {
		return errors.New("empty description or invalid name")
	}
	code, _, _ = qap.ParseDocumentCodes(code)
	if code == "" {
		return qap.ErrBadProjectCode
	}
	err := q.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucket([]byte("meta" + code))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucket([]byte(code))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return errors.New("error creating project, probably already exists: " + err.Error())
	}
	err = q.PutStructure(qap.Project{
		Code:        [3]byte{0: code[0], 1: code[1], 2: code[2]},
		Name:        name,
		Description: desc,
	})
	if err != nil {
		log.Println("while putting initial project structure:", err)
	}
	return nil
}

func (q *boltqap) NewDocument(doc document) error {
	info, err := doc.ValidateForAdmission()
	err = q.filter.Do(func(_ int, h qap.Header) error {
		if qap.HeadersEqual(h, info.Header) {
			return errors.New("document already exists:" + h.String())
		}
		return nil
	})
	if err != nil {
		return err
	}
	return q.addDoc(doc)
}

func (q *boltqap) NewMainDocument(doc document) (newdoc document, err error) {
	if doc.Revised.Before(doc.Created) {
		doc.Revised = time.Now() // ensure consistency
	}
	info, err := doc.ValidateForAdmission()
	doc.Number = 1 // Actual number assigned below.
	var maxCode int32
	q.filter.Do(func(i int, h qap.Header) error {
		if qap.HeaderCodesEqual(h, info.Header) {
			if h.Number > maxCode {
				maxCode = h.Number
			}
		}
		return nil
	})
	structure, err := q.GetStructure(doc.Project)
	if !structure.ContainsCode(info.Header) {
		return document{}, errors.New("equipment code is not defined in project structure. Must be added first.")
	}
	doc.Number = int(maxCode) + 1
	err = q.addDoc(doc)
	if err != nil {
		return document{}, err
	}
	return doc, nil
}

// addDoc adds the document with minimal validation. If document already
// exists it returns error.
func (q *boltqap) addDoc(doc document) error {
	hd, err := doc.Header()
	if err != nil {
		return err
	}
	err = q.filter.AddHeader(hd)
	if err != nil {
		return errors.New("unexpected error attempting to add document: " + err.Error())
	}
	return q.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(hd.Project()))
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
}

func (q *boltqap) DoProjects(fn func(structure qap.Project) error) error {
	err := q.db.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			if len(name) != len("metaLHC") {
				return nil
			}
			var structure qap.Project
			v := b.Get([]byte("structure"))
			err := json.Unmarshal(v, &structure)
			if err != nil {
				log.Println("error unmarshalling database structure " + string(name))
				return nil
			}
			return fn(structure)
		})
	})
	if errors.Is(err, ErrEndLookup) {
		return nil
	}
	return err
}

func (q *boltqap) DoProjectDocuments(project string, f func(d document) error) error {
	err := q.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(project))
		if b == nil {
			return fmt.Errorf("project %q not found", project)
		}
		return b.ForEach(func(k, v []byte) error {
			doc, err := docFromValue(v)
			if err != nil {
				log.Println("error reading document from database: ", err.Error())
				return nil
			}
			return f(doc)
		})
	})
	if err == nil || errors.Is(err, ErrEndLookup) {
		return nil
	}
	return err
}

func (q *boltqap) DoDocuments(f func(d document) error) error {
	err := q.db.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			if len(name) != 3 {
				return nil
			}
			return b.ForEach(func(k, v []byte) error {
				doc, err := docFromValue(v)
				if err != nil {
					log.Println("error reading document from database: ", err.Error())
					return nil
				}
				err = f(doc)
				if errors.Is(err, ErrEndLookup) {
					return ErrEndLookup
				}
				if err != nil {
					log.Println(err, string(k), string(v))
				}
				return err
			})
		})
	})
	if errors.Is(err, ErrEndLookup) {
		return nil
	}
	return err
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
			if len(name) != 3 {
				return nil
			}
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
				if errors.Is(err, ErrEndLookup) {
					break
				}
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

// FindDocument finds the document identically matching the header.
func (q *boltqap) FindDocument(target qap.Header) (doc document, err error) {
	err = target.Validate()
	if err != nil {
		return document{}, err
	}
	err = q.DoProjectDocuments(target.Project(), func(d document) error {
		h, err := d.Header()
		if err != nil {
			return fmt.Errorf("document %s has Header error: %s", d, err)
		}
		if qap.HeadersEqual(h, target) {
			doc = d
			return ErrEndLookup
		}
		return nil
	})
	return doc, err
}

func (q *boltqap) AddRevision(target qap.Header, newrev revision) error {
	err := newrev.Index.Validate()
	if err != nil {
		return err
	}
	doc, err := q.FindDocument(target)
	if err != nil {
		return err
	}
	incoming := newrev.Index
	latest := doc.Revision()
	min, maj := qap.AreSequential(latest, incoming)
	if !min && !maj {
		return errors.New("revision is not sequential")
	}
	doc.Revisions = append(doc.Revisions, newrev)
	return q.Update(doc)
}

func (q *boltqap) Update(d document) error {
	_, err := d.Info()
	if err != nil {
		return err
	}
	return q.db.Update(func(tx *bbolt.Tx) error {
		buck := tx.Bucket([]byte(d.Project))
		if buck == nil {
			return errors.New(d.Project + " project does not exist")
		}
		key := d.key()
		exist := buck.Get(key)
		if exist == nil {
			return errors.New(d.String() + " document does not exist in DB")
		}
		return buck.Put(key, d.value())
	})
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

func (q *boltqap) GetStructure(project string) (structure qap.Project, err error) {
	if len(project) != 3 {
		return structure, qap.ErrBadProjectCode
	}
	name := "meta" + project
	return structure, q.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(name))
		if b == nil {
			return errors.New("project metadata not found")
		}
		v := b.Get([]byte("structure"))
		return json.Unmarshal(v, &structure)
	})
}

func (q *boltqap) PutStructure(structure qap.Project) (err error) {
	str := structure.Project()
	if len(str) != 3 {
		return errors.New("bad project code")
	}
	metakey := []byte("meta" + str)
	tx, err := q.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Commit()
	b := tx.Bucket(metakey)
	if b == nil {
		if tx.Bucket([]byte(str)) != nil {
			_, err = tx.CreateBucket(metakey)
			log.Println("project exists, attempted to create missing metadata bucket. err:", err)
		} else {
			tx.Rollback()
		}
		return errors.New("project " + str + " metadata not found, try again?")
	}
	val, err := json.Marshal(structure)
	if err != nil {
		tx.Rollback()
		return err
	}
	key := []byte("structure")
	err = b.Put(key, val)
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}
