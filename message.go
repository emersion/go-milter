package milter

// Message represents a command sent from milter client
type Message struct {
	Code byte
	Data []byte
}

type ActionCode byte

const (
	ActAccept    ActionCode = 'a' // SMFIR_ACCEPT
	ActContinue  ActionCode = 'c' // SMFIR_CONTINUE
	ActDiscard   ActionCode = 'd' // SMFIR_DISCARD
	ActReject    ActionCode = 'r' // SMFIR_REJECT
	ActTempFail  ActionCode = 't' // SMFIR_TEMPFAIL
	ActReplyCode ActionCode = 'y' // SMFIR_REPLYCODE

	// [v6]
	ActSkip ActionCode = 's' // SMFIR_SKIP
)

type ModifyActCode byte

const (
	ActAddRcpt      ModifyActCode = '+' // SMFIR_ADDRCPT
	ActDelRcpt      ModifyActCode = '-' // SMFIR_DELRCPT
	ActReplBody     ModifyActCode = 'b' // SMFIR_ACCEPT
	ActAddHeader    ModifyActCode = 'h' // SMFIR_ADDHEADER
	ActChangeHeader ModifyActCode = 'm' // SMFIR_CHGHEADER
	ActInsertHeader ModifyActCode = 'i' // SMFIR_INSHEADER
	ActQuarantine   ModifyActCode = 'q' // SMFIR_QUARANTINE

	// [v6]
	ActChangeFrom ModifyActCode = 'e' // SMFIR_CHGFROM
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

	// [v6]
	CodeQuitNewConn Code = 'K' // SMFIC_QUIT_NC
)

const MaxBodyChunk = 65535

type ProtoFamily byte

const (
	FamilyUnknown ProtoFamily = 'U' // SMFIA_UNKNOWN
	FamilyUnix    ProtoFamily = 'L' // SMFIA_UNIX
	FamilyInet    ProtoFamily = '4' // SMFIA_INET
	FamilyInet6   ProtoFamily = '6' // SMFIA_INET6
)
