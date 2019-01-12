package main

func signum(number int) int {
	if number == 0 {
		return 0
	} else if number < 0 {
		return -1
	} else {
		return 1
	}
}
