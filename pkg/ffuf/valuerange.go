package ffuf

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ValueRange struct {
	Min, Max int64
}

func ValueRangeFromString(instr string) (ValueRange, error) {
	// Values are often written as "200, 301", on the command line and in config files.
	instr = strings.TrimSpace(instr)
	// is the value a range
	minmax := regexp.MustCompile(`^(\d+)-(\d+)$`).FindAllStringSubmatch(instr, -1)
	if minmax != nil {
		// yes
		minval, err := strconv.ParseInt(minmax[0][1], 10, 0)
		if err != nil {
			return ValueRange{}, fmt.Errorf("Invalid value: %s", minmax[0][1])
		}
		maxval, err := strconv.ParseInt(minmax[0][2], 10, 0)
		if err != nil {
			return ValueRange{}, fmt.Errorf("Invalid value: %s", minmax[0][2])
		}
		if minval >= maxval {
			return ValueRange{}, fmt.Errorf("Minimum has to be smaller than maximum")
		}
		return ValueRange{minval, maxval}, nil
	} else {
		// no, a single value or something else
		intval, err := strconv.ParseInt(instr, 10, 0)
		if err != nil {
			return ValueRange{}, fmt.Errorf("Invalid value: %s", instr)
		}
		return ValueRange{intval, intval}, nil
	}
}
