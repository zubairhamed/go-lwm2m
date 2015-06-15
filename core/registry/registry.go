package registry

import (
	. "github.com/zubairhamed/betwixt"
	"github.com/zubairhamed/betwixt/objectdefs/ipso"
	"github.com/zubairhamed/betwixt/objectdefs/oma"
)

func NewDefaultObjectRegistry() Registry {
	reg := NewObjectRegistry(&oma.LWM2MCoreObjects{}, &ipso.IPSOSmartObjects{})

	return reg
}

func NewObjectRegistry(s ...ObjectSource) Registry {
	reg := &ObjectRegistry{}
	reg.sources = []ObjectSource{}

	for _, o := range s {
		reg.Register(o)
	}
	return reg
}

type ObjectRegistry struct {
	sources []ObjectSource
}

func (m *ObjectRegistry) GetDefinition(n LWM2MObjectType) ObjectDefinition {
	for _, s := range m.sources {
		if s != nil {
			o := s.GetObject(n)
			if o != nil {
				return o
			}
		}
	}
	return nil
}

func (m *ObjectRegistry) GetDefinitions() []ObjectDefinition {
	defs := []ObjectDefinition{}

	for _, s := range m.sources {
		if s != nil {
			for _, v := range s.GetObjects() {
				defs = append(defs, v)
			}
		}
	}
	return defs
}

func (m *ObjectRegistry) Register(s ObjectSource) {
	s.Initialize()
	m.sources = append(m.sources, s)
}

func (m *ObjectRegistry) GetMandatory() []ObjectDefinition {
	mandatory := []ObjectDefinition{}

	for _, s := range m.sources {
		objs := s.GetObjects()
		for _, o := range objs {
			if o.IsMandatory() {
				mandatory = append(mandatory, o)
			}
		}
	}
	return mandatory
}