package comm

func NewResetCommand() Command {
	return Command{command: reset}
}

func NewClearLEDCommand(target int) Command {
	return Command{command: clearLED, target: target}
}

func NewSetLEDCommand(target int, color byte) Command {
	return Command{command: setLED, target: target, color: color}
}

func NewToggleLEDCommand(target int, on bool) Command {
	if on {
		return NewSetLEDCommand(target, '1');
	} else {
		return NewClearLEDCommand(target);
	}
}
