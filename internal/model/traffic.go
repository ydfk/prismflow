package model

import (
	"fmt"
	"regexp"
	"time"
	_ "time/tzdata"
)

func (t TrafficSettings) Normalized() TrafficSettings {
	if t.ResetDay == 0 {
		t.ResetDay = 1
	}
	if t.Timezone == "" {
		t.Timezone = "UTC"
	}
	return t
}
func (t TrafficSettings) Validate() error {
	t = t.Normalized()
	if t.MaxBytes < 0 || t.MaxBytes > 9000000000000000 || t.ResetDay < 1 || t.ResetDay > 31 {
		return fmt.Errorf("流量上限或重置日期无效")
	}
	if _, err := time.LoadLocation(t.Timezone); err != nil {
		return fmt.Errorf("重置时区无效")
	}
	if t.Interface != "" && !regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,64}$`).MatchString(t.Interface) {
		return fmt.Errorf("网络接口名称无效")
	}
	return nil
}
