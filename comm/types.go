package comm

type messageKind int
type commandKind int

// messageKind values
const (
	invalid = iota
	Ready
	KnobTurned
	ButtonPressed
	ButtonReleased
)

// commandKind
const (
	_ = iota
	reset
	clearLED
	setLED
)

type Message struct {
	Message messageKind
	Source  int
	Value   int
}

type Command struct {
	command commandKind
	target  int
	color   byte
}
