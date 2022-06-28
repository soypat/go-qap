package main

import (
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
	if d != dpiped {
		t.Error("piped document differs")
	}
}
