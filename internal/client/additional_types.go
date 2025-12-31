package client

import (
	"strconv"
	"strings"
)

type Amount float64

func (a *Amount) UnmarshalJSON(v []byte) error {
	vStr := strings.Trim(string(v), `"`)
	out, err := strconv.ParseFloat(vStr, 64)
	if err != nil {
		return err
	}
	*a = Amount(out)
	return nil
}
