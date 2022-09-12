package qap

import "testing"

func TestParseRevision(t *testing.T) {
	for _, test := range []struct {
		Input  string
		Expect Revision
	}{
		{
			Input: "Z.9-draft",
			Expect: Revision{
				Index: [2]byte{'Z', '9'},
			},
		},
		{
			Input: "A.1-draft",
			Expect: Revision{
				Index: [2]byte{'A', '1'},
			},
		},
		{
			Input: "B.9",
			Expect: Revision{
				Index: [2]byte{'B', '9'}, IsRelease: true,
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
