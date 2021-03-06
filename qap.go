package qap

import (
	"errors"
	"fmt"
	"strings"
)

const (
	minDocumentNumber   = 0
	maxDocumentNumber   = 999_999
	maxAttachmentNumber = 99
	lenP                = len(Header{}.ProjectCode)
	lenE                = len(Header{}.EquipmentCode)
	lenDT               = len(Header{}.DocumentTypeCode)
	// Maximum length of a document name string including code separators.
	maxHeaderLength = lenP + lenE + lenDT + 6 + 2 + 4

	lenHeader = 4 + lenP + lenE + lenDT + 1
)

var (
	ErrInvalidNumber         = fmt.Errorf("QAP number out of range 0..%d", maxDocumentNumber)
	ErrEmptyProjectCode      = errors.New("zero length project code")
	ErrEmptyEquipmentCode    = errors.New("zero length equipment code")
	ErrEmptyDocumentTypeCode = errors.New("zero length document type code")
	ErrEmptyAttachmentNumber = errors.New("zero length attachment number")
	ErrBadProjectCode        = fmt.Errorf("project code must be %d upper case characters", lenP)
	ErrBadEquipmentCode      = fmt.Errorf("equipment code must be 1..%d digits or/and upper case characters", lenE)
	ErrBadDocumentTypeCode   = fmt.Errorf("document type code must be 1..%d upper case characters", lenDT)
	ErrBadAttachmentNumber   = fmt.Errorf("attachment number must be 2 digits in range 0..%d", maxAttachmentNumber)

	ErrZeroTime         = errors.New("creation/revision time is zero")
	ErrBadRevisionIndex = errors.New("revision index must be two digits or an upper case character followed by a digit")
)

// ParseDocumentCodes is a helper function to extract document codes from
// human input.
func ParseDocumentCodes(documentName string) (project, equipment, docType string) {
	const safeLen = maxHeaderLength + 5
	if len(documentName) > safeLen {
		documentName = documentName[:safeLen]
	}
	splits := strings.SplitN(strings.ToUpper(strings.TrimSpace(documentName)), "-", 4)
	if len(splits) > 0 && len(splits[0]) == lenP && reUpper.MatchString(splits[0]) {
		project = splits[0]
	}
	if len(splits) > 1 && 0 < len(splits[1]) && len(splits[1]) <= lenE && reUpperDigit.MatchString(splits[1]) {
		equipment = splits[1]
	}
	if len(splits) > 2 && len(splits[2]) == lenDT && reUpper.MatchString(splits[2]) {
		docType = splits[2]
	}
	return project, equipment, docType
}
