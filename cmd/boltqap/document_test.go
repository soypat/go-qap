package main

import (
	"fmt"
	"testing"
	"time"
)

func TestDocumentRecords(t *testing.T) {
	now, err := time.Parse(timeKeyFormat, time.Now().Format(timeKeyFormat))
	if err != nil {
		t.Fatal("incorrect test:", err)
	}
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
		Version:       "A.1-draft",
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

	if err := assertDocEqual(d, dpiped); err != nil {
		t.Errorf("piped document differs: %s", err)
	}
}

func assertDocEqual(a, b document) error {
	if a.Version != b.Version {
		return fmt.Errorf("Version not equal %q, %q", a.Version, b.Version)
	}
	if a.Location != b.Location {
		return fmt.Errorf("Location not equal %q, %q", a.Location, b.Location)
	}
	if a.SubmittedBy != b.SubmittedBy {
		return fmt.Errorf("SubmittedBy not equal %q, %q", a.SubmittedBy, b.SubmittedBy)
	}
	if a.Number != b.Number {
		return fmt.Errorf("Number not equal %d, %d", a.Number, b.Number)
	}
	if a.FileExtension != b.FileExtension {
		return fmt.Errorf("FileExtension not equal %q, %q", a.FileExtension, b.FileExtension)
	}
	if a.Project != b.Project {
		return fmt.Errorf("project not equal %q, %q", a.Project, b.Project)
	}
	if a.DocType != b.DocType {
		return fmt.Errorf("DocType not equal %q, %q", a.DocType, b.DocType)
	}
	if a.Attachment != b.Attachment {
		return fmt.Errorf("Attachment not equal %d, %d", a.Attachment, b.Attachment)
	}
	if a.Created != b.Created {
		return fmt.Errorf("Created not equal %q, %q", a.Created, b.Created)
	}
	if a.Revised != b.Revised {
		return fmt.Errorf("Revised not equal %q, %q", a.Revised, b.Revised)
	}
	if a.Deleted != b.Deleted {
		return fmt.Errorf("deleted not equal %t, %t", a.Deleted, b.Deleted)
	}
	if a.HumanName != b.HumanName {
		return fmt.Errorf("HumanName not equal %q, %q", a.HumanName, b.HumanName)
	}
	if a.Equipment != b.Equipment {
		return fmt.Errorf("Equipment not equal %q, %q", a.Equipment, b.Equipment)
	}
	return nil
}
