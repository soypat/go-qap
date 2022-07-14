package qap

import "fmt"

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

func (p Project) String() string {
	return validQAPAlpha(p.Code[:])
}

func (f Family) String() string { return f.Name }

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
	if !isAlpha(sys) {
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
	if !isAlpha(s.Code) {
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
	return ErrBadEquipmentCode
}
