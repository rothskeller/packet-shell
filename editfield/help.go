package editfield

func (e *Editor) showHelp() {
	if e.help == "" {
		return
	}
	lines, _ := wrap(e.help, e.screenWidth-1)
	e.out(resetAndClearEOS, 0)
	for _, line := range lines {
		e.out(helpStyle, 0)
		e.out(line, 0)
		e.out(spaces[:e.screenWidth-len(line)-1], 0)
		e.out(normalStyle, 0)
		e.newline()
	}
}
