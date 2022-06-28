package qap

import (
	"encoding/binary"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reUpper      = regexp.MustCompile("^[A-Z]*$")
	reUpperDigit = regexp.MustCompile("^[A-Z0-9]*$")
)

// Header represents a unique document name according to LHC's
// Quality Assurance Plan (QAP). The API is designed so that
// the Header is immutable through method calls, thus only direct
// access by the user to its fields should mutate the underlying data.
type Header struct {
	// A user or system controlled number or a combination thereof.
	// Is 3 to 6 digits long. Referred to as EDMS number in QAP202.
	Number int32
	// Composed of 3 upper case characters
	ProjectCode [3]byte
	// Composed of 1 to 5 alphanumeric characters.
	EquipmentCode [5]byte
	// DocumentTypeCode identifies the purpose of the document.
	// Composed of 2 upper case characters.
	DocumentTypeCode [2]byte
	// For material attached to main document. The main document's attachment
	// number is 0.
	AttachmentNumber uint8
}

// String returns the Header's document name representation i.e. "SPS-PEC-HP-023.00".
// This function is deterministic and different valid QAP document String()
// returned values will not collide.
func (h Header) String() string {
	if h.Validate() != nil {
		return "<invalid header>"
	}
	if h.Number < 999 {
		return fmt.Sprintf("%s-%s-%s-%03d.%02d", h.Project(), h.Equipment(), h.DocumentType(), h.Number, h.AttachmentNumber)
	}
	return fmt.Sprintf("%s-%s-%s-%06d.%02d", h.Project(), h.Equipment(), h.DocumentType(), h.Number, h.AttachmentNumber)
}

// ParseHeader parses a complete unversioned QAP document string of the style
// "SPS-PEC-HP-0023.A1". The attachment number parsing may be omitted by setting
// ignoreAttachment to true setting attachment result to 0.
//
// This function is very careful of the input and will more readily return an error
// before forming a valid Header from ambiguous or unexpected input.
func ParseHeader(header string, ignoreAttachment bool) (Header, error) {
	h := Header{}
	if len(header) > maxHeaderLength {
		// Prevent long string attack.
		return h, errors.New("document name longer than maximum possible length")
	}
	splits := strings.SplitN(header, "-", 4)
	if len(splits) < 4 {
		return h, fmt.Errorf("expected document name to be split in 4 substrings at \"-\" characters. got %d", len(splits))
	}
	switch {
	case len(splits[0]) == 0:
		return h, ErrEmptyProjectCode
	case len(splits[1]) == 0:
		return h, ErrEmptyEquipmentCode
	case len(splits[2]) == 0:
		return h, ErrEmptyDocumentTypeCode
	case len(splits[0]) != lenP:
		return h, ErrBadProjectCode
	case len(splits[1]) > lenE:
		return h, ErrBadEquipmentCode
	case len(splits[2]) != lenDT:
		return h, ErrBadDocumentTypeCode
	}
	var attachment uint8
	numStr, attachStr, foundAttachment := strings.Cut(splits[3], ".")
	if !ignoreAttachment {
		if !foundAttachment {
			return h, errors.New("did not find attachment number in document name following period")
		}
		attachNum, err := strconv.Atoi(attachStr)
		if err != nil {
			return h, errors.New("parsing attachment number: " + err.Error())
		}
		if attachNum < 0 || attachNum > maxAttachmentNumber {
			return h, ErrBadAttachmentNumber
		}
		attachment = uint8(attachNum)
	}
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return h, errors.New("parsing document name number: " + err.Error())
	}
	if num < minDocumentNumber || num > maxDocumentNumber {
		return h, ErrInvalidNumber
	}

	copy(h.ProjectCode[:], splits[0])
	copy(h.EquipmentCode[:], splits[1])
	copy(h.DocumentTypeCode[:], splits[2])
	h.Number = int32(num)
	h.AttachmentNumber = attachment
	if err := h.Validate(); err != nil {
		return Header{}, err
	}
	return h, nil
}

// Project returns a valid QAP project code string or an empty string.
func (h Header) Project() string {
	project := validQAPAlpha(h.ProjectCode[:])
	if len(project) != lenP {
		return ""
	}
	return project
}

// Equipment returns a valid QAP equipment code string or an empty string.
func (h Header) Equipment() string {
	return validQAPAlphanum(h.EquipmentCode[:])
}

// DocumentType returns a valid QAP document type code string or an empty string.
func (h Header) DocumentType() string {
	docType := validQAPAlpha(h.DocumentTypeCode[:])
	if len(docType) != lenDT {
		return ""
	}
	return docType
}

// Validate tests Header for malformed data.
func (h Header) Validate() (err error) {
	proj := h.ProjectCode[:]
	proj = proj[:idxOfNullOrLen(proj)]
	equip := h.EquipmentCode[:]
	equip = equip[:idxOfNullOrLen(equip)]
	doctype := h.DocumentTypeCode[:]
	doctype = doctype[:idxOfNullOrLen(doctype)]
	switch {
	case h.AttachmentNumber > maxAttachmentNumber:
		err = ErrBadAttachmentNumber
	case !reUpper.Match(proj):
		err = ErrBadProjectCode
	case !reUpperDigit.Match(equip):
		err = ErrBadEquipmentCode
	case !reUpper.Match(doctype):
		err = ErrBadDocumentTypeCode
	case h.Number < minDocumentNumber || h.Number > maxDocumentNumber:
		err = ErrInvalidNumber
	case len(proj) == 0:
		err = ErrEmptyProjectCode
	case len(equip) == 0:
		err = ErrEmptyEquipmentCode
	case len(doctype) == 0:
		err = ErrEmptyDocumentTypeCode
	}
	if err != nil {
		return err
	}
	// Length errors
	switch {
	case len(proj) != lenP:
		err = ErrBadProjectCode
	case len(doctype) != lenDT:
		err = ErrBadDocumentTypeCode
	}
	return err
}

func validQAPAlphanum(b []byte) string {
	return string(b[:idxOfNonAlphanumOrLen(b)])
}

func validQAPAlpha(b []byte) string {
	return string(b[:idxOfNonAlphaOrLen(b)])
}

func idxOfNonAlphanumOrLen(b []byte) int {
	for i := range b {
		char := b[i]
		if !isAlphaNum(char) {
			return i
		}
	}
	return len(b)
}

func idxOfNullOrLen(b []byte) int {
	for i := range b {
		if b[i] == 0 {
			return i
		}
	}
	return len(b)
}

func idxOfNonAlphaOrLen(b []byte) int {
	for i := range b {
		char := b[i]
		if !isAlpha(char) {
			return i
		}
	}
	return len(b)
}

// isAlphaNum returns true if b is a digit or upper case ASCII code point.
func isAlphaNum(char byte) bool {
	return isNum(char) || isAlpha(char)
}

// isNum returns true if char is a digit ASCII code point.
func isNum(char byte) bool {
	return char^'0' < 10
}

// isAlphaNum returns true if b is upper case ASCII code point.
func isAlpha(char byte) bool {
	return 'A' <= char && char <= 'Z'
}

func (h Header) puts(b []byte) error {
	if len(b) < lenHeader {
		return errors.New("puts arg too short")
	}
	binary.LittleEndian.PutUint32(b, uint32(h.Number))
	copy(b[4:], h.ProjectCode[:])
	copy(b[4+lenP:], h.EquipmentCode[:])
	copy(b[4+lenP+lenE:], h.DocumentTypeCode[:])
	b[4+lenP+lenE+lenDT] = h.AttachmentNumber
	return nil
}

func headerGets(b []byte) (Header, error) {
	h := Header{}
	if len(b) < lenHeader {
		return h, errors.New("gets arg too short")
	}
	h.Number = int32(binary.LittleEndian.Uint32(b))
	copy(h.ProjectCode[:], b[4:])
	copy(h.EquipmentCode[:], b[4+lenP:])
	copy(h.DocumentTypeCode[:], b[4+lenP+lenE:])
	h.AttachmentNumber = b[4+lenP+lenE+lenDT]
	return h, nil
}

// HeaderCodesEqual tests project, equipment and document type codes of
// a and b are the same. If either a or b are invalid then HeaderCodesEqual
// returns false.
func HeaderCodesEqual(a, b Header) bool {
	if a.Validate() != nil || b.Validate() != nil {
		return false
	}
	return a.Project() == b.Project() && a.Equipment() == b.Equipment() &&
		a.DocumentType() == b.DocumentType()
}

// HeadersEqual tests headers for complete equality.
// If either a or b are invalid then HeadersEqual returns false.
func HeadersEqual(a, b Header) bool {
	return HeaderCodesEqual(a, b) && a.Number == b.Number &&
		a.AttachmentNumber == b.AttachmentNumber
}
