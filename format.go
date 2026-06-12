//
// Copyright (C) 2026 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/chalk
//

package chalk

import (
	"fmt"
	"strings"
	"time"
)

const indentUnit = "    " // 4 spaces per level

func indentStr(level int) string {
	if level <= 0 {
		return ""
	}
	return strings.Repeat(indentUnit, level)
}

// formatWallClock formats a wall-clock offset as "00m 00.0s".
func formatWallClock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalTenths := int(d.Milliseconds() / 100)
	secs := totalTenths / 10
	tenths := totalTenths % 10
	mins := secs / 60
	secs = secs % 60
	return fmt.Sprintf("%02dm %02d.%ds", mins, secs, tenths)
}

// formatElapsed formats a task duration as "00.0s", or "00.0m" when >= 100 s.
func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < 100*time.Second {
		totalTenths := int(d.Milliseconds() / 100)
		secs := totalTenths / 10
		tenths := totalTenths % 10
		return fmt.Sprintf("%02d.%ds", secs, tenths)
	}
	wholeMinutes := int(d.Minutes())
	tenthsMin := int(d.Seconds()/6) % 10
	return fmt.Sprintf("%02d.%dm", wholeMinutes, tenthsMin)
}
