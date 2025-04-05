package ffuf

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// optRange stores either a single float, in which case the value is stored in min and IsRange is false,
// or a range of floats, in which case IsRange is true
type optRange struct {
	Min      float64
	Max      float64
	IsRange  bool
	HasDelay bool
}

type optRangeJSON struct {
	Value string `json:"value"`
}

func (o *optRange) MarshalJSON() ([]byte, error) {
	value := ""
	if o.Min == o.Max {
		value = fmt.Sprintf("%.2f", o.Min)
	} else {
		value = fmt.Sprintf("%.2f-%.2f", o.Min, o.Max)
	}
	return json.Marshal(&optRangeJSON{
		Value: value,
	})
}

func (o *optRange) UnmarshalJSON(b []byte) error {
	var inc optRangeJSON
	err := json.Unmarshal(b, &inc)
	if err != nil {
		return err
	}
	return o.Initialize(inc.Value)
}

// Initialize sets up the optRange from string value
func (o *optRange) Initialize(value string) error {
	var err, err2 error
	d := strings.Split(value, "-")
	if len(d) > 2 {
		return fmt.Errorf("Delay needs to be either a single float: \"0.1\" or a range of floats, delimited by dash: \"0.1-0.8\"")
	} else if len(d) == 2 {
		o.IsRange = true
		o.HasDelay = true
		o.Min, err = strconv.ParseFloat(d[0], 64)
		o.Max, err2 = strconv.ParseFloat(d[1], 64)
		if err != nil || err2 != nil {
			return fmt.Errorf("Delay range min and max values need to be valid floats. For example: 0.1-0.5")
		}
	} else if len(value) > 0 {
		o.IsRange = false
		o.HasDelay = true
		o.Min, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("Delay needs to be either a single float: \"0.1\" or a range of floats, delimited by dash: \"0.1-0.8\"")
		}
	}
	return nil
}
