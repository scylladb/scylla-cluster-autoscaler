package mock_func

func Abs(x int) int {
	if x < 0 {
		return -x
	} else {
		return x
	}
}
