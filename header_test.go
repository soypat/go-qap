package qap

import "testing"

func TestParseInvalidDocumentName(t *testing.T) {
	for _, test := range []struct {
		Input string
	}{
		{
			Input: "SP-PEC-HP-001.00",
		},
		{
			Input: "SPS-UPPE1-T-001000.32",
		},
		{
			Input: "SPS-UPPE1-T-.32",
		},
		{
			Input: "SPS-UPPE1-T-2.3A",
		},
	} {
		h, err := ParseHeader(test.Input, false)
		if err == nil {
			t.Errorf("expected %q to error", test.Input)
		}
		if h != (Header{}) {
			t.Error("expected zero value for Header")
		}
	}
}

func TestParseDocumentName(t *testing.T) {
	for _, test := range []struct {
		Input  string
		Expect Header
	}{
		{
			Input: "SPS-PEC-HP-001.00",
			Expect: Header{
				ProjectCode:      [3]byte{'S', 'P', 'S'},
				EquipmentCode:    [5]byte{'P', 'E', 'C'},
				DocumentTypeCode: [2]byte{'H', 'P'},
				Number:           1,
				AttachmentNumber: 0,
			},
		},
		{
			Input: "SPS-UPPE1-TP-001000.32",
			Expect: Header{
				ProjectCode:      [3]byte{'S', 'P', 'S'},
				EquipmentCode:    [5]byte{'U', 'P', 'P', 'E', '1'},
				DocumentTypeCode: [2]byte{'T', 'P'},
				Number:           1000,
				AttachmentNumber: 32,
			},
		},
	} {
		h, err := ParseHeader(test.Input, false)
		if err != nil {
			t.Error(err)
		}
		if h != test.Expect {
			t.Errorf("expected result %q from %q got %q", test.Expect, test.Input, h.String())
		}
		if h.String() != test.Input {
			t.Errorf("expected result String() %q to be equal to input %q", h.String(), test.Input)
		}
		hReparse, err := ParseHeader(h.String(), false)
		if err != nil {
			t.Errorf("reparsing %q got %q: %s", h.String(), hReparse.String(), err)
		}
	}
}

func TestParseDocumentNameCodes(t *testing.T) {
	for _, test := range []struct {
		Input                                         string
		ExpectProject, ExpectEquipment, ExpectDocType string
	}{
		{
			Input:         "-sps1-hp-adasdsdas12321",
			ExpectProject: "", ExpectEquipment: "SPS1", ExpectDocType: "HP",
		},
		{
			Input:         "sPs",
			ExpectProject: "SPS",
		},
		{
			Input:         "--sp",
			ExpectDocType: "SP",
		},
	} {
		project, equip, docType := ParseDocumentCodes(test.Input)
		if project == "" && equip == "" && docType == "" {
			t.Error("nothing found")
			continue
		}
		if project != test.ExpectProject {
			t.Errorf("expected project %s, got %s", test.ExpectProject, project)
		}
		if equip != test.ExpectEquipment {
			t.Errorf("expected equipment %s, got %s", test.ExpectEquipment, equip)
		}
		if docType != test.ExpectDocType {
			t.Errorf("expected doc type %s, got %s", test.ExpectDocType, docType)
		}
	}
}

func FuzzParseDocumentName(f *testing.F) {
	f.Add("SPS-PEC-HP-1.99")
	f.Add("ZZZ-PEC2C-HP-001.55")
	f.Add("LHC-SIRP-HP-21001.00")
	f.Fuzz(func(t *testing.T, a string) {
		hd, err := ParseHeader(a, false)
		if err != nil {
			return
		}
		hdstr := hd.String()
		hdr, err := ParseHeader(hdstr, false)
		if err != nil {
			t.Fatalf("Parsing valid result %q from %q errored: %s", hdstr, a, err)
		}
		hdrstr := hdr.String()
		if hdstr != hdrstr {
			t.Fatalf("string output %q not equal to input %q", hdrstr, hdstr)
		}
	})
}

func FuzzParseDocumentCodes(f *testing.F) {
	f.Add("SPS-PEC-HP-1.99")
	f.Add("ZZZ-P2CEC-HP")
	f.Add("LHC-SIRP-HP-21001.00")
	f.Fuzz(func(t *testing.T, a string) {
		proj, equip, doc := ParseDocumentCodes(a)
		if proj == "" && equip == "" && doc == "" {
			return
		}
		full := proj + "-" + equip + "-" + doc
		if a[:len(full)] != full {
			t.Errorf("%q != %q", a[:len(full)], full)
		}
	})
}
