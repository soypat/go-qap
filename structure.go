package qap

import (
	"errors"
	"fmt"
)

// Project structure. Mostly for having ready for use
// representations of projects. Equipment code naming convention
// can be found at https://edms.cern.ch/ui/file/103369/3.2/LHC-PM-QA-204-32-00.pdf
// document LHC-PM-QA-204-32-00.

// Project represents the overlying project structure as outlined by
// LHC-PM-QA-202 and LHC-PM-QA-204.
type Project struct {
	Code        [3]byte // Project name (3 letters)
	Systems     []System
	Name        string
	Description string
}

// System represents the first letter of the equipment code, which indicates
// the system to which the equipment belongs.
type System struct {
	Code        byte
	Families    []Family
	Name        string
	Description string
}

// Family represents the second letter of the equipment code. It defines the
// family of the equipment within a given system.
type Family struct {
	Code        byte
	Types       []Type
	Name        string
	Description string
}

// Type represents the third letter in an equipment code and defines the
// type within a family of an equipment.
type Type struct {
	Code        byte
	Models      []Model
	Name        string
	Description string
}

// Model represents the fourth letter within an equipment code.
type Model struct {
	Code        byte
	Variants    []Variant
	Name        string
	Description string
}

// Variant represents the fifth letter within an equipment code.
type Variant struct {
	Code        byte
	Name        string
	Description string
}

func (p *Project) AddEquipmentCode(code, name, description string) error {
	code = validQAPAlpha([]byte(code))
	switch len(code) {
	case 1:
		return p.AddSystem(System{Code: code[0], Name: name, Description: description})
	case 2:
		return p.AddFamily(code[0], Family{Code: code[1], Name: name, Description: description})
	case 3:
		return p.AddType(code[0], code[1], Type{Code: code[2], Name: name, Description: description})
	}
	return errors.New("invalid code")
}

// Project returns the project code string. i.e. "LHC"
func (p Project) Project() string {
	project := validQAPAlpha(p.Code[:])
	if project == "" {
		return "invalid project code"
	}
	return project
}

func (p Project) String() string {
	return p.Project()
}

func (s System) String() string {
	if s.Name == "" {
		return string(s.Code)
	}
	return s.Name
}
func (f Family) String() string {
	if f.Name == "" {
		return string(f.Code)
	}
	return f.Name
}
func (t Type) String() string {
	if t.Name == "" {
		return string(t.Code)
	}
	return t.Name
}
func (m Model) String() string {
	if m.Name == "" {
		return string(m.Code)
	}
	return m.Name
}
func (v Variant) String() string {
	if v.Name == "" {
		return string(v.Code)
	}
	return v.Name
}

func (p *Project) AddSystem(sys System) error {
	if !isAlpha(sys.Code) {
		return ErrBadEquipmentCode
	}
	for i := range p.Systems {
		code := p.Systems[i].Code
		if code == sys.Code {
			return fmt.Errorf("system %s already exists for project %s", string(code), p)
		}
	}
	p.Systems = append(p.Systems, sys)
	return nil
}

func (p *Project) AddFamily(sys byte, family Family) error {
	if !isAlpha(sys) || !isAlpha(family.Code) {
		return ErrBadEquipmentCode
	}
	for i := range p.Systems {
		code := p.Systems[i].Code
		if code == sys {
			return p.Systems[i].AddFamily(family)
		}
	}
	return fmt.Errorf("system %s not found in project %s", string(sys), p)
}

func (s *System) AddFamily(family Family) error {
	if !isAlpha(family.Code) {
		return ErrBadEquipmentCode
	}
	for i := range s.Families {
		if s.Families[i].Code == family.Code {
			return fmt.Errorf("family %s already exists (%s)", string(s.Families[i].Code), s.Families[i])
		}
	}
	s.Families = append(s.Families, family)
	return nil
}

func (p *Project) AddType(sys, family byte, tp Type) error {
	if !isAlpha(sys) || !isAlpha(family) || !isAlpha(tp.Code) {
		return ErrBadEquipmentCode
	}
	for i := range p.Systems {
		if p.Systems[i].Code == sys {
			p.Systems[i].AddType(family, tp)
		}
	}
	return fmt.Errorf("system %s not found in project %s", string(sys), p)
}

func (s *System) AddType(family byte, tp Type) error {
	if !isAlpha(family) || !isAlpha(tp.Code) {
		return ErrBadEquipmentCode
	}
	for i := range s.Families {
		if s.Families[i].Code == family {
			s.Families[i].AddType(tp)
		}
	}
	return fmt.Errorf("family %s not found in system %s", string(family), s)
}

func (f *Family) AddType(tp Type) error {
	if !isAlpha(tp.Code) {
		return ErrBadEquipmentCode
	}
	for i := range f.Types {
		if f.Types[i].Code == tp.Code {
			return fmt.Errorf("type %s already exists for family %s", tp, f)
		}
	}
	f.Types = append(f.Types, tp)
	return nil
}

func (s System) Letter() string {
	if isAlpha(s.Code) {
		return string(s.Code)
	}
	return "<invalid system code>"
}
func (f Family) Letter() string {
	if isAlpha(f.Code) {
		return string(f.Code)
	}
	return "<invalid family code>"
}
func (t Type) Letter() string {
	if isAlpha(t.Code) {
		return string(t.Code)
	}
	return "<invalid type code>"
}
func (m Model) Letter() string {
	if isAlpha(m.Code) {
		return string(m.Code)
	}
	return "<invalid model code>"
}
func (v Variant) Letter() string {
	if isAlpha(v.Code) {
		return string(v.Code)
	}
	return "<invalid variant code>"
}

// ContainsCode returns true if header equipment code and project code
// exist inside the project structure. Does not check for validity of entire Header.
func (p Project) ContainsCode(hd Header) bool {
	if p.Code != hd.ProjectCode {
		return false
	}
	equip := hd.Equipment()
	if len(equip) == 0 {
		return false
	}
	for _, sys := range p.Systems {
		if equip[0] == sys.Code {
			return sys.contains(equip)
		}
	}
	return false
}

func (s System) contains(code string) bool {
	if len(code) == 1 {
		return true
	}
	for _, fam := range s.Families {
		if code[1] == fam.Code {
			return fam.contains(code)
		}
	}
	return false
}

func (f Family) contains(code string) bool {
	if len(code) == 2 {
		return true
	}
	for _, tp := range f.Types {
		if code[2] == tp.Code {
			return tp.contains(code)
		}
	}
	return false
}

func (tp Type) contains(code string) bool {
	if len(code) == 3 {
		return true
	}
	for _, m := range tp.Models {
		if code[3] == m.Code {
			return m.contains(code)
		}
	}
	return false
}

func (m Model) contains(code string) bool {
	if len(code) == 4 {
		return true
	}
	for _, v := range m.Variants {
		if code[4] == v.Code {
			return true
		}
	}
	return false
}
