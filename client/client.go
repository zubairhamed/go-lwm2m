package client

import (
	"errors"
	. "github.com/zubairhamed/betwixt"
	"github.com/zubairhamed/betwixt/core"
	"github.com/zubairhamed/betwixt/core/request"
	. "github.com/zubairhamed/canopus"
	. "github.com/zubairhamed/go-commons/network"
	"log"
	"net"
)

func NewDefaultClient(local string, remote string, registry Registry) *DefaultClient {
	localAddr, err := net.ResolveUDPAddr("udp", local)
	IfErrFatal(err)

	remoteAddr, err := net.ResolveUDPAddr("udp", remote)
	IfErrFatal(err)

	coapServer := NewServer(localAddr, remoteAddr)

	// Create Mandatory
	c := &DefaultClient{
		coapServer:     coapServer,
		enabledObjects: make(map[LWM2MObjectType]Object),
		registry: registry,
	}

	mandatory := registry.GetMandatory()
	for _, o := range mandatory {
		c.EnableObject(o.GetType(), nil)
	}

	return c
}

type DefaultClient struct {
	coapServer     *CoapServer
	registry       Registry
	enabledObjects map[LWM2MObjectType]Object
	path           string

	// Events
	evtOnStartup      FnOnStartup
	evtOnRead         FnOnRead
	evtOnWrite        FnOnWrite
	evtOnExecute      FnOnExecute
	evtOnRegistered   FnOnRegistered
	evtOnDeregistered FnOnDeregistered
	evtOnError        FnOnError
}

// Operations
func (c *DefaultClient) Register(name string) string {
	if len(name) > 10 {
		log.Fatal("Client#Register: Client name can not exceed 10 characters")
	}

	req := NewRequest(TYPE_CONFIRMABLE, POST, GenerateMessageId())

	req.SetStringPayload(core.BuildModelResourceStringPayload(c.enabledObjects))
	req.SetRequestURI("rd")
	req.SetUriQuery("ep", name)
	resp, err := c.coapServer.Send(req)

	path := ""
	if err != nil {
		log.Println(err)
	} else {
		PrintMessage(resp.GetMessage())

		path = resp.GetMessage().GetLocationPath()
	}
	c.path = path

	return path
}

func (c *DefaultClient) SetEnabler(t LWM2MObjectType, e ObjectEnabler) {
	_, ok := c.enabledObjects[t]
	if ok {
		c.enabledObjects[t].SetEnabler(e)
	}
}

func (c *DefaultClient) GetEnabledObjects() map[LWM2MObjectType]Object {
	return c.enabledObjects
}

func (c *DefaultClient) GetRegistry() Registry {
	return c.registry
}

func (c *DefaultClient) Deregister() {
	req := NewRequest(TYPE_CONFIRMABLE, DELETE, GenerateMessageId())

	req.SetRequestURI(c.path)
	resp, err := c.coapServer.Send(req)

	if err != nil {
		log.Println(err)
	} else {
		PrintMessage(resp.GetMessage())
	}
}

func (c *DefaultClient) Update() {

}

func (c *DefaultClient) AddResource() {

}

func (c *DefaultClient) AddObject() {

}

func (c *DefaultClient) UseRegistry(reg Registry) {
	c.registry = reg
}

func (c *DefaultClient) EnableObject(t LWM2MObjectType, e ObjectEnabler) error {
	_, ok := c.enabledObjects[t]
	if !ok {
		if c.registry == nil {
			return errors.New("No registry found/set")
		}
		c.enabledObjects[t] = core.NewObject(t, e, c.registry)

		return nil
	} else {
		return errors.New("Object already enabled")
	}
}

func (c *DefaultClient) AddObjectInstance(t LWM2MObjectType, instance int) error {
	o := c.enabledObjects[t]
	if o != nil {
		o.AddInstance(instance)

		return nil
 	}
	return errors.New("Attempting to add a nil instance")
}

func (c *DefaultClient) AddObjectInstances(t LWM2MObjectType, instances ...int) {
	for _, o := range instances {
		c.AddObjectInstance(t, o)
	}
}

func (c *DefaultClient) GetObject(n LWM2MObjectType) Object {
	return c.enabledObjects[n]
}

func (c *DefaultClient) validate() {

}

func (c *DefaultClient) Start() {
	c.validate()

	s := c.coapServer
	s.OnStartup(func(evt *Event) {
		if c.evtOnStartup != nil {
			c.evtOnStartup()
		}
	})

	s.NewRoute("{obj}/{inst}/{rsrc}", GET, c.handleReadRequest)
	s.NewRoute("{obj}/{inst}", GET, c.handleReadRequest)
	s.NewRoute("{obj}", GET, c.handleReadRequest)

	s.NewRoute("{obj}/{inst}/{rsrc}", PUT, c.handleWriteRequest)
	s.NewRoute("{obj}/{inst}", PUT, c.handleWriteRequest)

	s.NewRoute("{obj}/{inst}", DELETE, c.handleDeleteRequest)

	s.NewRoute("{obj}/{inst}/{rsrc}", POST, c.handleExecuteRequest)
	s.NewRoute("{obj}/{inst}", POST, c.handleCreateRequest)

	c.coapServer.Start()
}

func (c *DefaultClient) handleCreateRequest(r Request) Response {
	req := r.(*CoapRequest)
	attrResource := req.GetAttribute("rsrc")
	objectId := req.GetAttributeAsInt("obj")
	instanceId := req.GetAttributeAsInt("inst")

	var resourceId = -1

	if attrResource != "" {
		resourceId = req.GetAttributeAsInt("rsrc")
	}

	t := LWM2MObjectType(objectId)
	obj := c.GetObject(t)
	enabler := obj.GetEnabler()

	msg := NewMessageOfType(TYPE_ACKNOWLEDGEMENT, req.GetMessage().MessageId)
	msg.Token = req.GetMessage().Token
	msg.Payload = NewEmptyPayload()

	if enabler != nil {
		lwReq := request.Default(req, OPERATIONTYPE_CREATE)
		response := enabler.OnCreate(instanceId, resourceId, lwReq)
		msg.Code = response.GetResponseCode()
	} else {
		msg.Code = COAPCODE_405_METHOD_NOT_ALLOWED
	}
	return NewResponseWithMessage(msg)
}

func (c *DefaultClient) handleReadRequest(r Request) Response {
	req := r.(*CoapRequest)
	attrResource := req.GetAttribute("rsrc")
	objectId := req.GetAttributeAsInt("obj")
	instanceId := req.GetAttributeAsInt("inst")

	var resourceId = -1

	if attrResource != "" {
		resourceId = req.GetAttributeAsInt("rsrc")
	}

	t := LWM2MObjectType(objectId)
	obj := c.GetObject(t)
	enabler := obj.GetEnabler()

	msg := NewMessageOfType(TYPE_ACKNOWLEDGEMENT, req.GetMessage().MessageId)
	msg.Token = req.GetMessage().Token

	if enabler != nil {
		model := obj.GetModel()
		resource := model.GetResource(resourceId)

		if resource == nil {
			// TODO: Return TLV of Object Instance
			msg.Code = COAPCODE_404_NOT_FOUND
		} else {
			if !core.IsReadableResource(resource) {
				msg.Code = COAPCODE_405_METHOD_NOT_ALLOWED
			} else {
				lwReq := request.Default(req, OPERATIONTYPE_READ)
				response := enabler.OnRead(instanceId, resourceId, lwReq)

				val := response.GetResponseValue()
				msg.Code = response.GetResponseCode()
				msg.Payload = NewBytesPayload(val.GetBytes())
			}
		}
	} else {
		msg.Code = COAPCODE_405_METHOD_NOT_ALLOWED
	}
	return NewResponseWithMessage(msg)
}

func (c *DefaultClient) handleDeleteRequest(r Request) Response {
	req := r.(*CoapRequest)
	objectId := req.GetAttributeAsInt("obj")
	instanceId := req.GetAttributeAsInt("inst")

	t := LWM2MObjectType(objectId)
	enabler := c.GetObject(t).GetEnabler()

	msg := NewMessageOfType(TYPE_ACKNOWLEDGEMENT, req.GetMessage().MessageId)
	msg.Token = req.GetMessage().Token
	msg.Payload = NewEmptyPayload()

	if enabler != nil {
		lwReq := request.Default(req, OPERATIONTYPE_DELETE)

		response := enabler.OnDelete(instanceId, lwReq)
		msg.Code = response.GetResponseCode()
	} else {
		msg.Code = COAPCODE_405_METHOD_NOT_ALLOWED
	}
	return NewResponseWithMessage(msg)
}

func (c *DefaultClient) handleDiscoverRequest() {

}

func (c *DefaultClient) handleObserveRequest() {

}

func (c *DefaultClient) handleWriteRequest(r Request) Response {
	req := r.(*CoapRequest)
	attrResource := req.GetAttribute("rsrc")
	objectId := req.GetAttributeAsInt("obj")
	instanceId := req.GetAttributeAsInt("inst")

	log.Println(req.GetMessage().Payload)

	var resourceId = -1

	if attrResource != "" {
		resourceId = req.GetAttributeAsInt("rsrc")
	}

	t := LWM2MObjectType(objectId)
	obj := c.GetObject(t)
	enabler := obj.GetEnabler()

	msg := NewMessageOfType(TYPE_ACKNOWLEDGEMENT, req.GetMessage().MessageId)
	msg.Token = req.GetMessage().Token
	msg.Payload = NewEmptyPayload()

	if enabler != nil {
		model := obj.GetModel()
		resource := model.GetResource(resourceId)
		if resource == nil {
			// TODO Write to Object Instance
			msg.Code = COAPCODE_404_NOT_FOUND
		} else {
			if !core.IsWritableResource(resource) {
				msg.Code = COAPCODE_405_METHOD_NOT_ALLOWED
			} else {
				lwReq := request.Default(req, OPERATIONTYPE_WRITE)
				response := enabler.OnWrite(instanceId, resourceId, lwReq)
				msg.Code = response.GetResponseCode()
			}
		}
	} else {
		msg.Code = COAPCODE_404_NOT_FOUND
	}
	return NewResponseWithMessage(msg)
}

func (c *DefaultClient) handleExecuteRequest(r Request) Response {
	req := r.(*CoapRequest)
	attrResource := req.GetAttribute("rsrc")
	objectId := req.GetAttributeAsInt("obj")
	instanceId := req.GetAttributeAsInt("inst")

	var resourceId = -1

	if attrResource != "" {
		resourceId = req.GetAttributeAsInt("rsrc")
	}

	t := LWM2MObjectType(objectId)
	obj := c.GetObject(t)
	enabler := obj.GetEnabler()

	msg := NewMessageOfType(TYPE_ACKNOWLEDGEMENT, req.GetMessage().MessageId)
	msg.Token = req.GetMessage().Token
	msg.Payload = NewEmptyPayload()

	if enabler != nil {
		model := obj.GetModel()
		resource := model.GetResource(resourceId)
		if resource == nil {
			msg.Code = COAPCODE_404_NOT_FOUND
		}

		if !core.IsExecutableResource(resource) {
			msg.Code = COAPCODE_405_METHOD_NOT_ALLOWED
		} else {
			lwReq := request.Default(req, OPERATIONTYPE_EXECUTE)
			response := enabler.OnExecute(instanceId, resourceId, lwReq)
			msg.Code = response.GetResponseCode()
		}
	} else {
		msg.Code = COAPCODE_404_NOT_FOUND
	}

	return NewResponseWithMessage(msg)
}

// Events
func (c *DefaultClient) OnStartup(fn FnOnStartup) {
	c.evtOnStartup = fn
}

func (c *DefaultClient) OnRead(fn FnOnRead) {
	c.evtOnRead = fn
}

func (c *DefaultClient) OnWrite(fn FnOnWrite) {
	c.evtOnWrite = fn
}

func (c *DefaultClient) OnExecute(fn FnOnExecute) {
	c.evtOnExecute = fn
}

func (c *DefaultClient) OnRegistered(fn FnOnRegistered) {
	c.evtOnRegistered = fn
}

func (c *DefaultClient) OnDeregistered(fn FnOnDeregistered) {
	c.evtOnDeregistered = fn
}

func (c *DefaultClient) OnError(fn FnOnError) {
	c.evtOnError = fn
}
