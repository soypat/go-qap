package qap

import (
	"errors"
)

// HeaderFilter is an in-memory header filtering
// structure optimized for common search patterns.
type HeaderFilter struct {
	data   []Header
	number []int32
	// Projects are indexed by it's full name.
	projects [][lenP]byte
	// Equipment is indexed by individual characters.
	equipment [lenE][]byte
	// document type is indexed by individual characters.
	document   [lenDT][]byte
	attachment []uint8
	// deleted indicates if header at ith place has been removed from filter.
	deleted []bool
}

// NewHeaderFilter initializes a HeaderFilter with headers data.
func NewHeaderFilter(headers []Header) HeaderFilter {
	n := len(headers)
	if n == 0 {
		return HeaderFilter{}
	}
	hf := HeaderFilter{
		data:       make([]Header, n),
		projects:   make([][lenP]byte, n),
		attachment: make([]uint8, n),
		number:     make([]int32, n),
		deleted:    make([]bool, n),
	}
	for i := 0; i < lenE; i++ {
		hf.equipment[i] = make([]byte, n)
	}
	for i := 0; i < lenDT; i++ {
		hf.document[i] = make([]byte, n)
	}
	copy(hf.data, headers)
	for i, hd := range headers {
		hf.number[i] = hd.Number
		hf.projects[i] = hd.ProjectCode
		hf.attachment[i] = hd.AttachmentNumber
		for j := 0; j < lenE; j++ {
			hf.equipment[j][i] = hd.EquipmentCode[j]
		}
		for j := 0; j < lenDT; j++ {
			hf.document[j][i] = hd.DocumentTypeCode[j]
		}
	}
	return hf
}

// Len returns the amount of headers contained in filter.
func (hf *HeaderFilter) Len() int { return len(hf.data) }

// AddHeader adds a header to the filter.
func (hf *HeaderFilter) AddHeader(h Header) error {
	if err := h.Validate(); err != nil {
		return err
	}
	if hf.Has(h) {
		return errors.New("header already present")
	}
	hf.data = append(hf.data, h)
	hf.number = append(hf.number, h.Number)
	hf.projects = append(hf.projects, h.ProjectCode)
	hf.attachment = append(hf.attachment, h.AttachmentNumber)
	hf.deleted = append(hf.deleted, false)
	for j := 0; j < lenE; j++ {
		hf.equipment[j] = append(hf.equipment[j], h.EquipmentCode[j])
	}
	for j := 0; j < lenDT; j++ {
		hf.document[j] = append(hf.document[j], h.DocumentTypeCode[j])
	}
	return nil
}

// HumanQuery queries filter for n matches which are stored in dst. The total
// amount of matches found is totalFound.
func (hf *HeaderFilter) HumanQuery(dst []Header, query string, page int) (n, totalFound int) {
	if page < 0 {
		panic("page must be 0 or greater")
	}
	header, err := ParseHeader(query, false)
	if err == nil && hf.Has(header) {
		if len(dst) > 0 {
			dst[0] = header
			return 1, 1
		}
		return 0, 1
	}
	dataLen := hf.Len()
	proj, equip, docT := ParseDocumentCodes(query)
	matchProj := len(proj) == lenP
	matchEquip := equip != ""
	matchdocT := len(docT) == lenDT
	active := b2i(matchProj) + b2i(matchEquip) + b2i(matchdocT)
	if active == 0 {
		return 0, 0
	}
	found := 0
	added := 0
	for i := 0; i < dataLen; i++ {
		matches := 0
		searching := len(dst) != 0 && (found/len(dst) == page)
		if hf.deleted[i] {
			continue
		}
		if matchProj && hf.data[i].Project() == proj {
			matches++
		}
		if matchEquip {
			eq := hf.data[i].Equipment()
			if len(eq) >= len(equip) && eq[:len(equip)] == equip {
				matches++
			}
		}
		if matchdocT && hf.data[i].DocumentType() == docT {
			matches++
		}
		if matches >= active {
			if searching {
				dst[added] = hf.data[i]
				added++
			}
			found++
		}
	}
	return added, found
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (hf *HeaderFilter) Has(h Header) bool {
	n := hf.Len()
	if err := h.Validate(); err != nil {
		return false
	}
	for i := 0; i < n; i++ {
		if !hf.deleted[i] && HeadersEqual(hf.data[i], h) {
			return true
		}
	}
	return false
}

func (hf *HeaderFilter) Do(f func(i int, h Header) error) error {
	n := hf.Len()
	for i := 0; i < n; i++ {
		if !hf.deleted[i] {
			err := f(i, hf.data[i])
			if err != nil {
				return err
			}
		}
	}
	return nil
}
