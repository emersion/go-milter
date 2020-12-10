// Package milter provides an interface to implement milter mail filters
package milter

// OptAction sets which actions the milter wants to perform.
// Multiple options can be set using a bitmask.
type OptAction uint32

// set which actions the milter wants to perform
const (
	OptAddHeader    OptAction = 0x01
	OptChangeBody   OptAction = 0x02
	OptAddRcpt      OptAction = 0x04
	OptRemoveRcpt   OptAction = 0x08
	OptChangeHeader OptAction = 0x10
	OptQuarantine   OptAction = 0x20
	OptChangeFrom   OptAction = 0x40
)

// OptProtocol masks out unwanted parts of the SMTP transaction.
// Multiple options can be set using a bitmask.
type OptProtocol uint32

const (
	OptNoConnect  OptProtocol = 0x01
	OptNoHelo     OptProtocol = 0x02
	OptNoMailFrom OptProtocol = 0x04
	OptNoRcptTo   OptProtocol = 0x08
	OptNoBody     OptProtocol = 0x10
	OptNoHeaders  OptProtocol = 0x20
	OptNoEOH      OptProtocol = 0x40

	// [v6] MTA supports ActSkip.
	OptSkip OptProtocol = 0x400

	// [v6] milter will not send action response for following MTA messages.
	OptNoHeaderReply  OptProtocol = 0x80
	OptNoConnReply    OptProtocol = 0x1000
	OptNoHeloReply    OptProtocol = 0x2000
	OptNoMailReply    OptProtocol = 0x4000
	OptNoRcptReply    OptProtocol = 0x8000
	OptNoDataReply    OptProtocol = 0x10000
	OptNoUnknownReply OptProtocol = 0x20000
	OptNoEOHReply     OptProtocol = 0x40000
	OptNoBodyReply    OptProtocol = 0x80000
)
