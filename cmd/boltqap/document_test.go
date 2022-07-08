package main

import (
	"testing"
	"time"

	"github.com/soypat/go-qap"
)

func TestDocumentRecords(t *testing.T) {
	now, err := time.Parse(timeKeyFormat, time.Now().Format(timeKeyFormat))
	if err != nil {
		t.Fatal("incorrect test:", err)
	}
	rev, _ := qap.ParseRevision("B.2")
	d := document{
		Project:       "SPS",
		Equipment:     "HRC",
		DocType:       "HP",
		SubmittedBy:   "Pato",
		Number:        1,
		Attachment:    10,
		HumanName:     "syskeyd format \"sempre\"",
		FileExtension: ".CATPART",
		Location:      "system/d/cad",
		Revisions:     []revision{{Index: rev}},
		Created:       now,
		Revised:       now,
	}
	if _, err := d.Info(); err != nil {
		t.Fatal("test is incorrect:", err)
	}
	dpiped, err := docFromRecord(d.records(), false)
	if err != nil {
		t.Fatal(err)
	}
	assertDocEqual(t, d, dpiped)
}

func assertDocEqual(t *testing.T, a, b document) error {
	if len(a.Revisions) == len(b.Revisions) {
		for i := range a.Revisions {
			if a.Revisions[i] != b.Revisions[i] {
				t.Errorf("%dth revision not equal %s,%s", i, a.Revisions[i], b.Revisions[i])
			}
		}
	} else {
		t.Error("revisions lengths unequal")
	}
	if a.Revision() != b.Revision() {
		t.Errorf("Revision not equal %q, %q", a.Revision(), b.Revision())
	}
	if a.Location != b.Location {
		t.Errorf("Location not equal %q, %q", a.Location, b.Location)
	}
	if a.SubmittedBy != b.SubmittedBy {
		t.Errorf("SubmittedBy not equal %q, %q", a.SubmittedBy, b.SubmittedBy)
	}
	if a.Number != b.Number {
		t.Errorf("Number not equal %d, %d", a.Number, b.Number)
	}
	if a.FileExtension != b.FileExtension {
		t.Errorf("FileExtension not equal %q, %q", a.FileExtension, b.FileExtension)
	}
	if a.Project != b.Project {
		t.Errorf("project not equal %q, %q", a.Project, b.Project)
	}
	if a.DocType != b.DocType {
		t.Errorf("DocType not equal %q, %q", a.DocType, b.DocType)
	}
	if a.Attachment != b.Attachment {
		t.Errorf("Attachment not equal %d, %d", a.Attachment, b.Attachment)
	}
	if a.Created != b.Created {
		t.Errorf("Created not equal %q, %q", a.Created, b.Created)
	}
	if a.Revised != b.Revised {
		t.Errorf("Revised not equal %q, %q", a.Revised, b.Revised)
	}
	if a.Deleted != b.Deleted {
		t.Errorf("deleted not equal %t, %t", a.Deleted, b.Deleted)
	}
	if a.HumanName != b.HumanName {
		t.Errorf("HumanName not equal %q, %q", a.HumanName, b.HumanName)
	}
	if a.Equipment != b.Equipment {
		t.Errorf("Equipment not equal %q, %q", a.Equipment, b.Equipment)
	}
	return nil
}
