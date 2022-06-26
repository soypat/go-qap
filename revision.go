package qap

import (
	"errors"
	"fmt"
	"strings"
)

const (
	_draftStr         = "-draft"
	maxRevisionLength = 3 + len(_draftStr)
)

type Revision struct {
	// Index is the document revision index and is
	// composed by two digits separated by a dot (or alphanumeric characters).
	Index [2]byte
	// When registered new documents are given revision index 0.1 or 0.A followed
	// by the text "-draft".
	// Prior to release, new revisions of draft documents are given the revision
	// index 0.1, 0.2, 0.3 or using alphanumeric revision index: 0.A, 0.B, 0.C.
	//
	// In the case of a minor change only the second digit/character of the
	// revision index is incremented, for example from 1.0 to 1.1.
	//
	// In the case of a major change the first digit of the revision index
	// is incremented while the second is set to 0
	// (or A if using alphanumeric revision index)
	IsRelease bool
}

func ParseRevision(revision string) (Revision, error) {
	if len(revision) < 3 {
		return Revision{}, errors.New("revision string must be at least length 3")
	}
	if len(revision) > maxRevisionLength {
		revision = revision[:maxRevisionLength]
	}
	major, minor, ok := strings.Cut(revision, ".")
	if len(major) != 1 {
		return Revision{}, errors.New("major revision index must be length 1")
	}
	if !ok || len(minor) < 1 {
		return Revision{}, errors.New("minor revision index not found")
	}
	if len(minor) != 1 && minor[1:] != _draftStr {
		return Revision{}, errors.New("expected minor revision index of length 1 followed by nothing or \"" + _draftStr + "\"")
	}
	r := Revision{Index: [2]byte{major[0], minor[0]}, IsRelease: minor[1:] == ""}
	if err := r.Validate(); err != nil {
		return Revision{}, err
	}
	return r, nil
}

// String returns the revision index as a string. i.e. "0.1-draft" or "2.A"
// If the DocInfo's revision index is invalid it returns a constant string.
func (d Revision) String() string {
	if d.Validate() != nil {
		return "<invalid revision index>"
	}
	appendStr := _draftStr
	if d.IsRelease {
		appendStr = ""
	}
	return fmt.Sprintf("%s.%s%s", string(d.Index[0]), string(d.Index[1]), appendStr)
}

// Validate tests the Revision is valid and returns ErrBadRevisionIndex if it is not.
func (d Revision) Validate() error {
	alphaNum1 := isAlphaNum(d.Index[1])
	if !alphaNum1 || isAlphaNum(d.Index[1]) && !isAlphaNum(d.Index[0]) ||
		!isNum(d.Index[0]) {
		return ErrBadRevisionIndex
	}
	return nil
}

// IncrementMinor returns the DocInfo with it's minor version incremented by
// one and IsReleased field set to isRelease argument.
func (d Revision) IncrementMinor(isRelease bool) (Revision, error) {
	if err := d.Validate(); err != nil {
		return Revision{}, err
	}
	if d.Index[1] == 'Z' || d.Index[1] == '9' {
		return Revision{}, errors.New("revision minor index overflow")
	}
	d.Index[1]++
	d.IsRelease = isRelease
	return d, nil
}

// IncrementMajor returns the DocInfo with it's major version incremented by
// one and IsReleased field set to isRelease argument.
func (d Revision) IncrementMajor(isRelease bool) (Revision, error) {
	if err := d.Validate(); err != nil {
		return Revision{}, err
	}
	if d.Index[0] == 'Z' || d.Index[0] == '9' && isNum(d.Index[1]) {
		return Revision{}, errors.New("revision major index overflow")
	}
	if isAlpha(d.Index[1]) {
		d.Index[1] = 'A'
		if d.Index[0] == '9' {
			d.Index[0] = 'A' - 1
		}
	} else {
		d.Index[1] = '0'
	}
	d.Index[0]++
	d.IsRelease = isRelease
	return d, nil
}
