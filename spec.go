package rtnl

func stringSat(value, spec string) bool {
	if spec == "" {
		return true
	}

	return value == spec
}
