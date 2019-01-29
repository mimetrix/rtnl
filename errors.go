package rtnl

import (
	"strings"
)

func IsNotFound(err error) bool {

	return strings.Contains(err.Error(), "not found")

}
