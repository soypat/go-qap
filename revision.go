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

// NewRevision returns the first version draft revision "A.1-draft".
func NewRevision() Revision {
	return Revision{
		Index:     [2]byte{'A', '1'},
		IsRelease: false,
	}
}

// Revision holds version code information of a document.
//
// When registered new documents should be given revision index A.1 and a non
// released status.
//
// Prior to release, new revisions of draft documents are given the revision
// index A.1, A.2, A.3 and so on.
//
// In the case of a minor change only the second digit/character of the
// revision index is incremented, for example from C.0 to C.1 or B.3 to B.4.
//
// In the case of a major change the first digit of the revision index
// is incremented while the second is set to 0.
// (or A if using alphanumeric revision index)
// Example:
//  A.4 -> B.0
type Revision struct {
	// Index is the document revision index and is
	// composed by two digits separated by a dot (or alphanumeric characters).
	Index [2]byte

	IsRelease bool
}

// ParseRevision creates a Revision from a formatted string
// i.e. "B.3-draft", "A.1"
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

// String returns the revision index as a string. i.e. "A.1-draft" or "A.2"
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
	if d.Index == (Revision{}).Index {
		return errors.New("revision not initialized")
	}
	if !isAlpha(d.Index[0]) || !isNum(d.Index[1]) {
		return ErrBadRevisionIndex
	}
	if d.Index == [2]byte{'A', '0'} {
		return errors.New("first revision must have non-zero minor index")
	}
	if d.Index == [2]byte{'A', '1'} && d.IsRelease {
		return errors.New("first revision must be draft")
	}
	return nil
}

// IncrementMinor returns the DocInfo with it's minor version incremented by
// one and IsReleased field set to isRelease argument.
func (d Revision) IncrementMinor(isRelease bool) (Revision, error) {
	if err := d.Validate(); err != nil {
		return Revision{}, err
	}
	if d.Index[1] == '9' {
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
	if d.Index[0] == 'Z' {
		return Revision{}, errors.New("revision major index overflow")
	}
	d.Index[0]++
	d.IsRelease = isRelease
	return d, nil
}

// AreSequential tests whether b follows a as a revision, indicating whether
// the increment between the two revisions is
//  - A minor revision, which can be either
//    - A draft to release increment (i.e. A.1-draft -> A.1)
//    - A minor index increment (i.e. C.2 -> C.3 or C.2 -> C.3-draft)
//  - A major revision (i.e. A.3 -> B.1 or A.3 -> B.1-draft)
// It returns false for both minor and major if revisions are not in ascending
// order, they are not a single increment apart or if they are invalid revisions.
func AreSequential(a, b Revision) (minor, major bool) {
	if a.Index == b.Index {
		// Take care of draft to release increment case.
		return !a.IsRelease && b.IsRelease, false
	}
	nextMinor, err := a.IncrementMinor(a.IsRelease)
	if err != nil {
		return false, false
	}
	nextMajor, err := a.IncrementMajor(a.IsRelease)
	if err != nil {
		return false, false
	}
	if b.Index == nextMinor.Index {
		return true, false
	} else if b.Index == nextMajor.Index {
		return false, true
	}
	return false, false
}
