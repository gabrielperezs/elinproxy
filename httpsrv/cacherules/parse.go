package cacherules

import (
	"log"
	"strconv"
	"time"
)

// Parse will read the string values and convert to time.Duration
func (ir *InternalRules) Parse() error {
	var err error
	ir.RespStatusCodeTTL = make(map[int]time.Duration, 0)
	for k, v := range ir.RespStatusCodeTTLString {
		c, _ := strconv.Atoi(k)
		if c < 100 {
			continue
		}
		ir.RespStatusCodeTTL[c], err = time.ParseDuration(v)
		if err != nil {
			log.Printf("D: Cache RespStatusCodeTTL %d: %s", c, v)
			return err
		}
	}

	ir.RespContentTypeTTL = make(map[string]time.Duration, 0)
	for k, v := range ir.RespContentTypeTTLString {
		ir.RespContentTypeTTL[k], err = time.ParseDuration(v)
		if err != nil {
			log.Printf("D: Cache RespContentTypeTTL %s: %s", v, k)
			return err
		}
	}

	return nil
}
