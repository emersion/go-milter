package milter

// Message represents a command sent from milter client
type Message struct {
	Code byte
	Data []byte
}

type ActionCode byte

const (
	ActAccept    ActionCode = 'a'
	ActContinue  ActionCode = 'c'
	ActDiscard   ActionCode = 'd'
	ActReject    ActionCode = 'r'
	ActTempFail  ActionCode = 't'
	ActReplyCode ActionCode = 'y'
	ActSkip      ActionCode = 's'
)

type ModifyActCode byte

const (
	ActAddRcpt      ModifyActCode = '+'
	ActDelRcpt      ModifyActCode = '-'
	ActReplBody     ModifyActCode = 'b'
	ActAddHeader    ModifyActCode = 'h'
	ActChangeHeader ModifyActCode = 'm'
	ActInsertHeader ModifyActCode = 'i'
	ActChangeFrom   ModifyActCode = 'e'
	ActQuarantine   ModifyActCode = 'q'
)

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
	CodeData   Code = 'T' // SMFIC_DATA
	CodeQuitNC Code = 'K' // SMFIC_QUIT_NC
)

const MaxBodyChunk = 65535

type ProtoFamily byte

const (
	FamilyUnknown ProtoFamily = 'U' // SMFIA_UNKNOWN
	FamilyUnix    ProtoFamily = 'L' // SMFIA_UNIX
	FamilyInet    ProtoFamily = '4' // SMFIA_INET
	FamilyInet6   ProtoFamily = '6' // SMFIA_INET6
)
