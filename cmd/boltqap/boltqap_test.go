package main

import (
	"os"
	"testing"
	"time"

	"github.com/soypat/go-qap"
)

func TestBoltKey(t *testing.T) {
	for i := 0; i < 200; i++ {
		now := time.Now().Round(time.Millisecond)
		for _, d := range []time.Duration{0, time.Millisecond, time.Microsecond, time.Nanosecond} {
			for add := time.Duration(0); add < 1000; add += 21 {
				newt := now.Add(d*add + add)
				expect, _ := time.Parse(timeKeyFormat, newt.Format(timeKeyFormat))
				b := boltKey(newt)
				got, err := time.Parse(timeKeyFormat, string(b))
				if err != nil {
					t.Fatal(err)
				}
				if got != expect {
					t.Errorf("got not rounded %s %s", got, expect)
				}
			}
		}
	}
}

func TestBoltStore(t *testing.T) {
	const testFile = "qap_test.db"
	q, err := OpenBoltQAP(testFile, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(testFile)
	defer q.Close()
	rev, err := qap.ParseRevision("C.3")
	if err != nil {
		t.Fatal(err)
	}
	time1 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	doc1 := document{
		Project:       "SPS",
		Equipment:     "A",
		DocType:       "HP",
		SubmittedBy:   "pato",
		Number:        1,
		Location:      "/1/",
		HumanName:     "human name",
		FileExtension: "catpart",
		Revisions:     []revision{{Index: qap.NewRevision(), Description: "first"}, {Index: rev, Description: "second"}},
		Created:       time1,
		Revised:       time1,
	}
	err = q.CreateProject(doc1.Project)
	if err != nil {
		t.Fatal(err)
	}
	err = q.addDoc(doc1)
	if err != nil {
		t.Fatal(err)
	}
	hd, err := doc1.Header()
	got, err := q.FindDocument(hd)
	if err != nil {
		t.Fatal(err)
	}
	assertDocEqual(t, got, doc1)
}

func TestDoDocumentRange(t *testing.T) {
	const testFile = "qap_test.db"
	q, err := OpenBoltQAP(testFile, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(testFile)
	defer q.Close()
	rev, err := qap.ParseRevision("C.3")
	if err != nil {
		t.Fatal(err)
	}
	time1 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	time2 := time1.AddDate(10, 0, 0)
	time3 := time2.AddDate(10, 0, 0)
	doc1 := document{
		Project:       "SPS",
		Equipment:     "A",
		DocType:       "HP",
		SubmittedBy:   "pato",
		Number:        1,
		Location:      "/1/",
		HumanName:     "human name",
		FileExtension: "catpart",
		Revisions:     []revision{{Index: rev}},
		Created:       time1,
		Revised:       time1,
	}
	doc2 := doc1
	doc2.Equipment = "B"
	doc2.Created = time2
	doc3 := doc1
	doc3.Equipment = "C"
	doc3.Created = time3
	err = q.CreateProject("SPS")
	if err != nil {
		t.Fatal(err)
	}
	err = q.ImportDocuments([]document{doc1, doc2, doc3})
	if err != nil {
		t.Error(err)
	}

	t.Run("incrementing time", func(t *testing.T) {
		done := 0
		q.DoDocumentsRange(time1, time3, func(d document) error {
			if done == 0 && d.Created != time1 {
				t.Errorf("first document expected creation time %s, got %s", time1, d.Created)
			}
			if done == 1 && d.Created != time2 {
				t.Errorf("second document expected creation time %s, got %s", time2, d.Created)
			}
			if done == 2 && d.Created != time3 {
				t.Errorf("third document expected creation time %s, got %s", time3, d.Created)
			}
			done++
			return nil
		})
		if done != 3 {
			t.Error("expected 3 documents to be iterated, got ", done)
		}
	})

	t.Run("decrementing time", func(t *testing.T) {
		done := 0
		q.DoDocumentsRange(time3, time1, func(d document) error {
			if done == 0 && d.Created != time3 {
				t.Errorf("first document expected creation time %s, got %s", time3, d.Created)
			}
			if done == 1 && d.Created != time2 {
				t.Errorf("second document expected creation time %s, got %s", time2, d.Created)
			}
			if done == 2 && d.Created != time1 {
				t.Errorf("third document expected creation time %s, got %s", time1, d.Created)
			}
			done++
			return nil
		})
		if done != 3 {
			t.Error("expected 3 documents to be iterated, got ", done)
		}
	})

	t.Run("decrementing time OOB", func(t *testing.T) {
		done := 0
		q.DoDocumentsRange(time3.AddDate(0, 0, 1), time1, func(d document) error {
			if done == 0 && d.Created != time3 {
				t.Errorf("first document expected creation time %s, got %s", time3, d.Created)
			}
			if done == 1 && d.Created != time2 {
				t.Errorf("second document expected creation time %s, got %s", time2, d.Created)
			}
			if done == 2 && d.Created != time1 {
				t.Errorf("third document expected creation time %s, got %s", time1, d.Created)
			}
			done++
			return nil
		})
		if done != 3 {
			t.Error("expected 3 documents to be iterated, got ", done)
		}
	})
}
