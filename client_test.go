package milter

import (
	"bytes"
	"net"
	nettextproto "net/textproto"
	"reflect"
	"testing"

	"github.com/emersion/go-message/textproto"
)

func init() {
	// HACK: claim to support v6 in server for tests
	serverProtocolVersion = 6
}

type MockMilter struct {
	ConnResp Response
	ConnMod  func(m *Modifier)
	ConnErr  error

	HeloResp Response
	HeloMod  func(m *Modifier)
	HeloErr  error

	MailResp Response
	MailMod  func(m *Modifier)
	MailErr  error

	RcptResp Response
	RcptMod  func(m *Modifier)
	RcptErr  error

	HdrResp Response
	HdrMod  func(m *Modifier)
	HdrErr  error

	HdrsResp Response
	HdrsMod  func(m *Modifier)
	HdrsErr  error

	BodyChunkResp Response
	BodyChunkMod  func(m *Modifier)
	BodyChunkErr  error

	BodyResp Response
	BodyMod  func(m *Modifier)
	BodyErr  error

	AbortMod func(m *Modifier)
	AbortErr error

	// Info collected during calls.
	Host   string
	Family string
	Port   uint16
	Addr   net.IP

	HeloValue string
	From      string
	Rcpt      []string
	Hdr       nettextproto.MIMEHeader

	Chunks [][]byte
}

func (mm *MockMilter) Connect(host string, family string, port uint16, addr net.IP, m *Modifier) (Response, error) {
	if mm.ConnMod != nil {
		mm.ConnMod(m)
	}
	mm.Host = host
	mm.Family = family
	mm.Port = port
	mm.Addr = addr
	return mm.ConnResp, mm.ConnErr
}

func (mm *MockMilter) Helo(name string, m *Modifier) (Response, error) {
	if mm.HeloMod != nil {
		mm.HeloMod(m)
	}
	mm.HeloValue = name
	return mm.HeloResp, mm.HeloErr
}

func (mm *MockMilter) MailFrom(from string, m *Modifier) (Response, error) {
	if mm.MailMod != nil {
		mm.MailMod(m)
	}
	mm.From = from
	return mm.MailResp, mm.MailErr
}

func (mm *MockMilter) RcptTo(rcptTo string, m *Modifier) (Response, error) {
	if mm.RcptMod != nil {
		mm.RcptMod(m)
	}
	mm.Rcpt = append(mm.Rcpt, rcptTo)
	return mm.RcptResp, mm.RcptErr
}

func (mm *MockMilter) Header(name string, value string, m *Modifier) (Response, error) {
	if mm.HdrMod != nil {
		mm.HdrMod(m)
	}
	return mm.HdrResp, mm.HdrErr
}

func (mm *MockMilter) Headers(h nettextproto.MIMEHeader, m *Modifier) (Response, error) {
	if mm.HdrsMod != nil {
		mm.HdrsMod(m)
	}
	mm.Hdr = h
	return mm.HdrsResp, mm.HdrsErr
}

func (mm *MockMilter) BodyChunk(chunk []byte, m *Modifier) (Response, error) {
	if mm.BodyChunkMod != nil {
		mm.BodyChunkMod(m)
	}
	mm.Chunks = append(mm.Chunks, chunk)
	return mm.BodyChunkResp, mm.BodyChunkErr
}

func (mm *MockMilter) Body(m *Modifier) (Response, error) {
	if mm.BodyMod != nil {
		mm.BodyMod(m)
	}
	return mm.BodyResp, mm.BodyErr
}

func (mm *MockMilter) Abort(m *Modifier) error {
	if mm.AbortMod != nil {
		mm.AbortMod(m)
	}
	return mm.AbortErr
}

func TestMilterClient_UsualFlow(t *testing.T) {
	mm := MockMilter{
		ConnResp:      RespContinue,
		HeloResp:      RespContinue,
		MailResp:      RespContinue,
		RcptResp:      RespContinue,
		HdrResp:       RespContinue,
		HdrsResp:      RespContinue,
		BodyChunkResp: RespContinue,
		BodyResp:      RespContinue,
		BodyMod: func(m *Modifier) {
			m.AddHeader("X-Bad", "very")
			m.ChangeHeader(1, "Subject", "***SPAM***")
			m.Quarantine("very bad message")
		},
	}
	s := Server{
		NewMilter: func() Milter {
			return &mm
		},
		Actions: OptAddHeader | OptChangeHeader,
	}
	defer s.Close()
	local, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go s.Serve(local)

	cl := NewClientWithOptions("tcp", local.Addr().String(), ClientOptions{
		ActionMask: OptAddHeader | OptChangeHeader | OptQuarantine,
	})
	defer cl.Close()
	session, err := cl.Session()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	assertAction := func(act *Action, err error, expectCode ActionCode) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		if act.Code != expectCode {
			t.Fatal("Unexpectedcode:", act.Code)
		}
	}

	act, err := session.Conn("host", FamilyInet, 25565, "172.0.0.1")
	assertAction(act, err, ActContinue)
	if mm.Host != "host" {
		t.Fatal("Wrong host:", mm.Host)
	}
	if mm.Family != "tcp4" {
		t.Fatal("Wrong family:", mm.Family)
	}
	if mm.Port != 25565 {
		t.Fatal("Wrong port:", mm.Port)
	}
	if mm.Addr.String() != "172.0.0.1" {
		t.Fatal("Wrong IP:", mm.Addr)
	}

	if err := session.Macros(CodeHelo, "tls_version", "very old"); err != nil {
		t.Fatal("Unexpected error", err)
	}

	act, err = session.Helo("helo_host")
	assertAction(act, err, ActContinue)
	if mm.HeloValue != "helo_host" {
		t.Fatal("Wrong helo value:", mm.HeloValue)
	}

	act, err = session.Mail("from@example.org", []string{"A=B"})
	assertAction(act, err, ActContinue)
	if mm.From != "from@example.org" {
		t.Fatal("Wrong MAIL FROM:", mm.From)
	}

	act, err = session.Rcpt("to1@example.org", []string{"A=B"})
	assertAction(act, err, ActContinue)
	act, err = session.Rcpt("to2@example.org", []string{"A=B"})
	assertAction(act, err, ActContinue)
	if !reflect.DeepEqual(mm.Rcpt, []string{"to1@example.org", "to2@example.org"}) {
		t.Fatal("Wrong recipients:", mm.Rcpt)
	}

	hdr := textproto.Header{}
	hdr.Add("From", "from@example.org")
	hdr.Add("To", "to@example.org")
	hdr.Add("x-empty-header", "")
	act, err = session.Header(hdr)
	assertAction(act, err, ActContinue)
	if len(mm.Hdr) != 3 {
		t.Fatal("Unexpected header length:", len(mm.Hdr))
	}
	if val := mm.Hdr.Get("From"); val != "from@example.org" {
		t.Fatal("Wrong From header:", val)
	}
	if val := mm.Hdr.Get("To"); val != "to@example.org" {
		t.Fatal("Wrong To header:", val)
	}
	if val := mm.Hdr.Get("x-empty-header"); val != "" {
		t.Fatal("Wrong To header:", val)
	}

	modifyActs, act, err := session.BodyReadFrom(bytes.NewReader(bytes.Repeat([]byte{'A'}, 128000)))
	assertAction(act, err, ActContinue)

	if len(mm.Chunks) != 2 {
		t.Fatal("Wrong amount of body chunks received")
	}
	if len(mm.Chunks[0]) > 65535 {
		t.Fatal("Too big first chunk:", len(mm.Chunks[0]))
	}
	if totalLen := len(mm.Chunks[0]) + len(mm.Chunks[1]); totalLen < 128000 {
		t.Fatal("Some body bytes lost:", totalLen)
	}

	expected := []ModifyAction{
		{
			Code:        ActAddHeader,
			HeaderName:  "X-Bad",
			HeaderValue: "very",
		},
		{
			Code:        ActChangeHeader,
			HeaderIndex: 1,
			HeaderName:  "Subject",
			HeaderValue: "***SPAM***",
		},
		{
			Code:   ActQuarantine,
			Reason: "very bad message",
		},
	}

	if !reflect.DeepEqual(modifyActs, expected) {
		t.Fatalf("Wrong modify actions, got %+v", modifyActs)
	}
}

func TestMilterClient_AbortFlow(t *testing.T) {
	macros := make(map[string]string)
	mm := MockMilter{
		ConnResp: RespContinue,
		HeloResp: RespContinue,
		HeloMod: func(m *Modifier) {
			macros = m.Macros
		},
		AbortMod: func(m *Modifier) {
			macros = m.Macros
		},
	}
	s := Server{
		NewMilter: func() Milter {
			return &mm
		},
		Actions: OptAddHeader | OptChangeHeader,
	}
	defer s.Close()
	local, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go s.Serve(local)

	cl := NewClientWithOptions("tcp", local.Addr().String(), ClientOptions{
		ActionMask: OptAddHeader | OptChangeHeader | OptQuarantine,
	})
	defer cl.Close()
	session, err := cl.Session()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	assertAction := func(act *Action, err error, expectCode ActionCode) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		if act.Code != expectCode {
			t.Fatal("Unexpectedcode:", act.Code)
		}
	}

	act, err := session.Conn("host", FamilyInet, 25565, "172.0.0.1")
	assertAction(act, err, ActContinue)
	if mm.Host != "host" {
		t.Fatal("Wrong host:", mm.Host)
	}
	if mm.Family != "tcp4" {
		t.Fatal("Wrong family:", mm.Family)
	}
	if mm.Port != 25565 {
		t.Fatal("Wrong port:", mm.Port)
	}
	if mm.Addr.String() != "172.0.0.1" {
		t.Fatal("Wrong IP:", mm.Addr)
	}

	if err := session.Macros(CodeHelo, "tls_version", "very old"); err != nil {
		t.Fatal("Unexpected error", err)
	}

	act, err = session.Helo("helo_host")
	assertAction(act, err, ActContinue)
	if mm.HeloValue != "helo_host" {
		t.Fatal("Wrong helo value:", mm.HeloValue)
	}
	if v, ok := macros["tls_version"]; !ok || v != "very old" {
		t.Fatal("Wrong tls_version macro value:", v)
	}

	err = session.Abort()
	if err != nil {
		t.Fatal(err)
	}

	// Validate macro values are preserved for the abort callback
	if v, ok := macros["tls_version"]; !ok || v != "very old" {
		t.Fatal("Wrong tls_version macro value: ", v)
	}

	act, err = session.Helo("repeated_helo_host")
	assertAction(act, err, ActContinue)
	if mm.HeloValue != "repeated_helo_host" {
		t.Fatal("Wrong helo value:", mm.HeloValue)
	}
	if len(macros["tls_version"]) != 0 {
		t.Fatal("Unexpected macro data:", macros)
	}
}
