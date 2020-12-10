// Package milter provides an interface to implement milter mail filters
package milter

// OptAction sets which actions the milter wants to perform.
// Multiple options can be set using a bitmask.
type OptAction uint32

// Set which actions the milter wants to perform.
const (
	OptAddHeader    OptAction = 1 << 0 // SMFIF_ADDHDRS
	OptChangeBody   OptAction = 1 << 1 // SMFIF_CHGBODY
	OptAddRcpt      OptAction = 1 << 2 // SMFIF_ADDRCPT
	OptRemoveRcpt   OptAction = 1 << 3 // SMFIF_DELRCPT
	OptChangeHeader OptAction = 1 << 4 // SMFIF_CHGHDRS
	OptQuarantine   OptAction = 1 << 5 // SMFIF_QUARANTINE

	// [v6]
	OptChangeFrom      OptAction = 1 << 6 // SMFIF_CHGFROM
	OptAddRcptWithArgs OptAction = 1 << 7 // SMFIF_ADDRCPT_PAR
	OptSetSymList      OptAction = 1 << 8 // SMFIF_SETSYMLIST
)

// OptProtocol masks out unwanted parts of the SMTP transaction.
// Multiple options can be set using a bitmask.
type OptProtocol uint32

const (
	OptNoConnect  OptProtocol = 1 << 0 // SMFIP_NOCONNECT
	OptNoHelo     OptProtocol = 1 << 1 // SMFIP_NOHELO
	OptNoMailFrom OptProtocol = 1 << 2 // SMFIP_NOMAIL
	OptNoRcptTo   OptProtocol = 1 << 3 // SMFIP_NORCPT
	OptNoBody     OptProtocol = 1 << 4 // SMFIP_NOBODY
	OptNoHeaders  OptProtocol = 1 << 5 // SMFIP_NOHDRS
	OptNoEOH      OptProtocol = 1 << 6 // SMFIP_NOEOH
	OptNoUnknown  OptProtocol = 1 << 8 // SMFIP_NOUNKNOWN
	OptNoData     OptProtocol = 1 << 9 // SMFIP_NODATA

	// [v6] MTA supports ActSkip
	OptSkip OptProtocol = 1 << 10 // SMFIP_SKIP
	// [v6] Filter wants rejected RCPTs
	OptRcptRej OptProtocol = 1 << 11 // SMFIP_RCPT_REJ

	// Milter will not send action response for the following MTA messages
	OptNoHeaderReply OptProtocol = 1 << 7 // SMFIP_NR_HDR, SMFIP_NOHREPL
	// [v6]
	OptNoConnReply    OptProtocol = 1 << 12 // SMFIP_NR_CONN
	OptNoHeloReply    OptProtocol = 1 << 13 // SMFIP_NR_HELO
	OptNoMailReply    OptProtocol = 1 << 14 // SMFIP_NR_MAIL
	OptNoRcptReply    OptProtocol = 1 << 15 // SMFIP_NR_RCPT
	OptNoDataReply    OptProtocol = 1 << 16 // SMFIP_NR_DATA
	OptNoUnknownReply OptProtocol = 1 << 17 // SMFIP_NR_UNKN
	OptNoEOHReply     OptProtocol = 1 << 18 // SMFIP_NR_EOH
	OptNoBodyReply    OptProtocol = 1 << 19 // SMFIP_NR_BODY

	// [v6]
	OptHeaderLeadingSpace OptProtocol = 1 << 20 // SMFIP_HDR_LEADSPC
)
