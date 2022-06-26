package qap

import (
	"errors"
	"fmt"
	"time"
)

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
// i.e "LHC-PM-QA-202.00 rev 1.2"
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
	if d.Creation == (time.Time{}) {
		return errors.New("got zero value for time of creation")
	} else if d.RevisionTime == (time.Time{}) {
		return errors.New("got zero value for time of revision")
	}
	if err := d.Revision.Validate(); err != nil {
		return err
	}
	return nil
}
