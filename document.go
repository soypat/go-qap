package qap

import (
	"fmt"
	"strings"
	"time"
)

const (
	_revStr               = " rev "
	maxDocumentNameLength = maxHeaderLength + len(_revStr) + maxRevisionLength
)

// ParseDocumentName parses a full document name of the style
// "LHC-PM-QA-202.00 rev B.2" and returns the corresponding header.
//
// If there is no " rev " text in the string the header is parsed
// and a new A.1 revision is returned.
func ParseDocumentName(docName string) (Header, Revision, error) {
	const safeLen = maxDocumentNameLength
	if len(docName) > safeLen {
		docName = docName[:safeLen]
	}
	header, rev, foundRev := strings.Cut(docName, _revStr)
	hd, err := ParseHeader(header, false)
	if err != nil {
		return hd, Revision{}, err
	}
	if !foundRev {
		return hd, NewRevision(), nil
	}
	r, err := ParseRevision(rev)
	if err != nil {
		return Header{}, Revision{}, err
	}
	return hd, r, nil
}

// DocInfo defines a document's type, naming and revision as specified
// by CERN's Quality Assurance Plan along with some helper data relating to time.
type DocInfo struct {
	Header
	Revision Revision
	// Time document was created.
	Creation time.Time
	// Time revision index was last incremented.
	RevisionTime time.Time
}

// String returns the document information as a string. If DocInfo is invalid
// it return a constant non-empty string.
// i.e "LHC-PM-QA-202.00 rev C.2"
func (d DocInfo) String() string {
	if err := d.Validate(); err != nil {
		return "<invalid document>"
	}
	return fmt.Sprintf("%s rev %s", d.Header.String(), d.Revision.String())
}

// Validate tests DocInfo for malformed data.
func (d DocInfo) Validate() error {
	if err := d.Header.Validate(); err != nil {
		return err
	}
	if err := d.Revision.Validate(); err != nil {
		return err
	}
	if d.Creation == (time.Time{}) || d.RevisionTime == (time.Time{}) {
		return ErrZeroTime
	}
	return nil
}
