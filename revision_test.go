package qap

import "testing"

func TestParseRevision(t *testing.T) {
	for _, test := range []struct {
		Input  string
		Expect Revision
	}{
		{
			Input: "9.Z-draft",
			Expect: Revision{
				Index: [2]byte{'9', 'Z'},
			},
		},
		{
			Input: "0.1-draft",
			Expect: Revision{
				Index: [2]byte{'0', '1'},
			},
		},
		{
			Input: "3.0",
			Expect: Revision{
				Index: [2]byte{'3', '0'}, IsRelease: true,
			},
		},
		{
			Input: "9.A",
			Expect: Revision{
				Index: [2]byte{'9', 'A'}, IsRelease: true,
			},
		},
		{
			Input: "0.A",
			Expect: Revision{
				Index: [2]byte{'0', 'A'}, IsRelease: true,
			},
		},
	} {
		rev, err := ParseRevision(test.Input)
		if err != nil {
			t.Error(err)
		}
		revStr := rev.String()
		if rev != test.Expect {
			t.Errorf("expected result %q from %q got %q", test.Expect, test.Input, revStr)
		}
		if revStr != test.Input {
			t.Errorf("expected result String() %q to be equal to input %q", revStr, test.Input)
		}
		revReparse, err := ParseRevision(revStr)
		if err != nil {
			t.Errorf("reparsing %q got %q: %s", revStr, revReparse.String(), err)
		}
	}
}
