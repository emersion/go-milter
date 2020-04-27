package milter

// Message represents a command sent from milter client
type Message struct {
	Code byte
	Data []byte
}

// Define milter response codes
// TODO(foxcpp): Get rid of these in favor of Act* constants.
const (
	accept     = 'a'
	continue_  = 'c'
	discard    = 'd'
	quarantine = 'q'
	reject     = 'r'
	tempFail   = 't'
	replyCode  = 'y'
)

// Milter protocol version implemented by this package.
//
// Note: Not exported as we might want to support multiple versions
// transparently in the future.
const protocolVersion = 2

type Code byte

const (
	CodeOptNeg Code = 'O' // SMFIC_OPTNEG
	CodeMacro  Code = 'D' // SMFIC_MACRO
	CodeConn   Code = 'C' // SMFIC_CONNECT
	CodeQuit   Code = 'Q' // SMFIC_QUIT
	CodeHelo   Code = 'H' // SMFIC_HELO
	CodeMail   Code = 'M' // SMFIC_MAIL
	CodeRcpt   Code = 'R' // SMFIC_RCPT
	CodeHeader Code = 'L' // SMFIC_HEADER
	CodeEOH    Code = 'N' // SMFIC_EOH
	CodeBody   Code = 'B' // SMFIC_BODY
	CodeEOB    Code = 'E' // SMFIC_BODYEOB
	CodeAbort  Code = 'A' // SMFIC_ABORT
)

const MaxBodyChunk = 65535

type ProtoFamily byte

const (
	FamilyUnknown ProtoFamily = 'U' // SMFIA_UNKNOWN
	FamilyUnix    ProtoFamily = 'L' // SMFIA_UNIX
	FamilyInet    ProtoFamily = '4' // SMFIA_INET
	FamilyInet6   ProtoFamily = '6' // SMFIA_INET6
)
