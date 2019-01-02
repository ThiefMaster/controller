package comm

func MakeReset() Command {
	return Command{command: reset}
}

func MakeClearLED(target int) Command {
	return Command{command: clearLED, target: target}
}

func MakeSetLED(target int, color byte) Command {
	return Command{command: setLED, target: target, color: color}
}
