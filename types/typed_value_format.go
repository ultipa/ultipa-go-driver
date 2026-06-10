package types

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FormatValue serializes a TypedValue to its canonical string representation.
// This is used for CSV export and human-readable output.
func (tv *TypedValue) FormatValue() (string, error) {
	if tv.IsNull {
		return "", nil
	}

	val, err := tv.ToGo()
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}

	switch tv.Type {
	case PropertyTypeNull:
		return "", nil

	case PropertyTypeBool:
		if v, ok := val.(bool); ok {
			if v {
				return "true", nil
			}
			return "false", nil
		}

	case PropertyTypeInt32:
		return fmt.Sprintf("%d", val.(int32)), nil

	case PropertyTypeUint32:
		return fmt.Sprintf("%d", val.(uint32)), nil

	case PropertyTypeInt64:
		return fmt.Sprintf("%d", val.(int64)), nil

	case PropertyTypeUint64:
		return fmt.Sprintf("%d", val.(uint64)), nil

	case PropertyTypeFloat:
		return strconv.FormatFloat(float64(val.(float32)), 'f', -1, 32), nil

	case PropertyTypeDouble:
		return strconv.FormatFloat(val.(float64), 'f', -1, 64), nil

	case PropertyTypeString, PropertyTypeText:
		return val.(string), nil

	case PropertyTypeDecimal:
		return val.(Decimal).Value, nil

	case PropertyTypeDate:
		d := val.(GqldbDate)
		return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day), nil

	case PropertyTypeLocalTime:
		lt := val.(LocalTime)
		return formatTime(lt.Hour, lt.Minute, lt.Second, lt.Nanosecond), nil

	case PropertyTypeZonedTime:
		zt := val.(ZonedTime)
		return formatTime(zt.Hour, zt.Minute, zt.Second, zt.Nanosecond) + formatOffset(zt.OffsetMinutes), nil

	case PropertyTypeTimestamp:
		t := val.(time.Time)
		return formatDateTime(t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond()), nil

	case PropertyTypeLocalDatetime, PropertyTypeDatetime:
		ldt := val.(LocalDateTime)
		t := ldt.Time
		return formatDateTime(t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond()), nil

	case PropertyTypeZonedDatetime:
		zdt := val.(ZonedDateTime)
		t := zdt.Time
		return formatDateTime(t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond()) + formatOffset(zdt.OffsetMinutes), nil

	case PropertyTypeYearToMonth:
		ytm := val.(YearToMonth)
		return formatYearToMonth(ytm.Months), nil

	case PropertyTypeDayToSecond:
		dts := val.(DayToSecond)
		return formatDayToSecond(dts.Seconds, dts.Nanoseconds), nil

	case PropertyTypePoint:
		p := val.(Point)
		return fmt.Sprintf("point({latitude: %v, longitude: %v})", p.Latitude, p.Longitude), nil

	case PropertyTypePoint3D:
		p := val.(Point3D)
		return fmt.Sprintf("point({x: %v, y: %v, z: %v})", p.X, p.Y, p.Z), nil

	case PropertyTypeVector:
		v := val.(Vector)
		parts := make([]string, len(v.Values))
		for i, f := range v.Values {
			parts[i] = strconv.FormatFloat(float64(f), 'f', -1, 32)
		}
		return "[" + strings.Join(parts, ",") + "]", nil

	case PropertyTypeBlob:
		// Base64 (StdEncoding) — the canonical text form for binary in
		// CSV/JSON; round-trips with parseBlob in NewTypedValueFromString.
		if b, ok := val.([]byte); ok {
			return base64.StdEncoding.EncodeToString(b), nil
		}
	}

	return fmt.Sprintf("%v", val), nil
}

// NewTypedValueFromString parses a string into a TypedValue for the given target type.
// This is used for CSV import where all values arrive as strings.
func NewTypedValueFromString(s string, targetType PropertyType) (*TypedValue, error) {
	if s == "" {
		return &TypedValue{Type: targetType, IsNull: true}, nil
	}

	switch targetType {
	case PropertyTypeBool:
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "true" || s == "1" {
			return NewTypedValue(true)
		}
		return NewTypedValue(false)

	case PropertyTypeInt32:
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %q as int32: %w", s, err)
		}
		return NewTypedValue(int32(v))

	case PropertyTypeUint32:
		v, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %q as uint32: %w", s, err)
		}
		return NewTypedValue(uint32(v))

	case PropertyTypeInt64:
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %q as int64: %w", s, err)
		}
		return NewTypedValue(int64(v))

	case PropertyTypeUint64:
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %q as uint64: %w", s, err)
		}
		return NewTypedValue(uint64(v))

	case PropertyTypeFloat:
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %q as float: %w", s, err)
		}
		return NewTypedValue(float32(v))

	case PropertyTypeDouble:
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %q as double: %w", s, err)
		}
		return NewTypedValue(float64(v))

	case PropertyTypeString, PropertyTypeText:
		return NewTypedValue(s)

	case PropertyTypeDecimal:
		return NewTypedValue(Decimal{Value: s})

	case PropertyTypeDate:
		d, err := parseDate(s)
		if err != nil {
			return nil, err
		}
		return NewTypedValue(d)

	case PropertyTypeLocalTime:
		lt, err := parseLocalTime(s)
		if err != nil {
			return nil, err
		}
		return NewTypedValue(lt)

	case PropertyTypeZonedTime:
		zt, err := parseZonedTime(s)
		if err != nil {
			return nil, err
		}
		return NewTypedValue(zt)

	case PropertyTypeLocalDatetime:
		ldt, err := parseLocalDateTime(s)
		if err != nil {
			return nil, err
		}
		return NewTypedValue(ldt)

	case PropertyTypeDatetime:
		ldt, err := parseLocalDateTime(s)
		if err != nil {
			return nil, err
		}
		return NewTypedValue(Datetime{Time: ldt.Time})

	case PropertyTypeZonedDatetime:
		zdt, err := parseZonedDateTime(s)
		if err != nil {
			return nil, err
		}
		return NewTypedValue(zdt)

	case PropertyTypeTimestamp:
		t, err := parseTimestamp(s)
		if err != nil {
			return nil, err
		}
		return NewTypedValue(t)

	case PropertyTypeYearToMonth:
		ytm, err := parseYearToMonth(s)
		if err != nil {
			return nil, err
		}
		return NewTypedValue(ytm)

	case PropertyTypeDayToSecond:
		dts, err := parseDayToSecond(s)
		if err != nil {
			return nil, err
		}
		return NewTypedValue(dts)

	case PropertyTypePoint:
		p, err := parsePoint(s)
		if err != nil {
			return nil, err
		}
		return NewTypedValue(p)

	case PropertyTypePoint3D:
		p, err := parsePoint3D(s)
		if err != nil {
			return nil, err
		}
		return NewTypedValue(p)

	case PropertyTypeVector:
		v, err := parseVector(s)
		if err != nil {
			return nil, err
		}
		return NewTypedValue(v)

	case PropertyTypeBlob:
		b, err := parseBlob(s)
		if err != nil {
			return nil, err
		}
		return NewTypedValue(b)

	default:
		return nil, fmt.Errorf("unsupported target type for string parsing: %d", targetType)
	}
}

// =============================================================================
// Format helpers
// =============================================================================

func formatTime(hour, minute, second uint8, nanosecond uint32) string {
	base := fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
	if nanosecond != 0 {
		return base + "." + formatNanos(nanosecond)
	}
	return base
}

func formatDateTime(year, month, day, hour, minute, second, nanosecond int) string {
	base := fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", year, month, day, hour, minute, second)
	if nanosecond != 0 {
		return base + "." + formatNanos(uint32(nanosecond))
	}
	return base
}

func formatOffset(offsetMinutes int16) string {
	if offsetMinutes == 0 {
		return "+00:00"
	}
	sign := "+"
	mins := int(offsetMinutes)
	if mins < 0 {
		sign = "-"
		mins = -mins
	}
	return fmt.Sprintf("%s%02d:%02d", sign, mins/60, mins%60)
}

func formatNanos(n uint32) string {
	s := fmt.Sprintf("%09d", n)
	return strings.TrimRight(s, "0")
}

func formatYearToMonth(months int32) string {
	if months == 0 {
		return "P0M"
	}
	sign := ""
	m := months
	if m < 0 {
		sign = "-"
		m = -m
	}
	years := m / 12
	rem := m % 12
	result := sign + "P"
	if years > 0 {
		result += fmt.Sprintf("%dY", years)
	}
	if rem > 0 || years == 0 {
		result += fmt.Sprintf("%dM", rem)
	}
	return result
}

func formatDayToSecond(seconds int64, nanoseconds uint32) string {
	if seconds == 0 && nanoseconds == 0 {
		return "PT0S"
	}
	negative := seconds < 0
	absSecs := uint64(seconds)
	if negative {
		absSecs = uint64(-seconds)
	}
	days := absSecs / 86400
	rem := absSecs % 86400
	hours := rem / 3600
	rem = rem % 3600
	minutes := rem / 60
	secs := rem % 60

	result := "P"
	if negative {
		result = "-P"
	}
	if days > 0 {
		result += fmt.Sprintf("%dD", days)
	}
	if hours > 0 || minutes > 0 || secs > 0 || nanoseconds > 0 {
		result += "T"
		if hours > 0 {
			result += fmt.Sprintf("%dH", hours)
		}
		if minutes > 0 {
			result += fmt.Sprintf("%dM", minutes)
		}
		if secs > 0 || nanoseconds > 0 {
			if nanoseconds > 0 {
				result += fmt.Sprintf("%d.%sS", secs, formatNanos(nanoseconds))
			} else {
				result += fmt.Sprintf("%dS", secs)
			}
		}
	}
	return result
}

// =============================================================================
// Input normalization helpers
// =============================================================================

// removeOffset strips any trailing timezone offset or Z from a time string.
// "14:30:00+08:00" → "14:30:00", "14:30:00Z" → "14:30:00", "14:30:00 +08:00" → "14:30:00"
// Only looks in the last 10 chars to avoid matching date hyphens like "1993-05-06".
func removeOffset(s string) string {
	if strings.HasSuffix(s, "Z") {
		return s[:len(s)-1]
	}
	minIdx := len(s) - 10
	if minIdx < 1 {
		minIdx = 1
	}
	for i := len(s) - 1; i >= minIdx; i-- {
		if s[i] == '+' || s[i] == '-' {
			if len(s)-i >= 5 {
				cutAt := i
				if cutAt > 0 && s[cutAt-1] == ' ' {
					cutAt--
				}
				return s[:cutAt]
			}
		}
	}
	return s
}

// normalizeOffset collapses space before offset and converts Z to +00:00.
// "14:30:00 +08:00" → "14:30:00+08:00", "14:30:00Z" → "14:30:00+00:00"
func normalizeOffset(s string) string {
	if strings.HasSuffix(s, "Z") {
		return s[:len(s)-1] + "+00:00"
	}
	for i := len(s) - 1; i > 1; i-- {
		if s[i] == '+' || s[i] == '-' {
			if len(s)-i >= 5 && s[i-1] == ' ' {
				return s[:i-1] + s[i:]
			}
			break
		}
	}
	return s
}

// =============================================================================
// Parse helpers
// =============================================================================

func parseDate(s string) (GqldbDate, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return GqldbDate{}, fmt.Errorf("cannot parse %q as date (expected yyyy-MM-dd): %w", s, err)
	}
	return GqldbDate{Year: int16(t.Year()), Month: uint8(t.Month()), Day: uint8(t.Day())}, nil
}

func parseLocalTime(s string) (LocalTime, error) {
	s = removeOffset(s)
	hour, minute, second, nano, err := parseTimeComponents(s)
	if err != nil {
		return LocalTime{}, fmt.Errorf("cannot parse %q as local_time (expected HH:mm:ss[.n]): %w", s, err)
	}
	return LocalTime{Hour: hour, Minute: minute, Second: second, Nanosecond: nano}, nil
}

func parseZonedTime(s string) (ZonedTime, error) {
	s = normalizeOffset(s)
	timePart, offset, err := splitOffset(s)
	if err != nil {
		return ZonedTime{}, fmt.Errorf("cannot parse %q as zoned_time (expected HH:mm:ss[.n]+HH:MM): %w", s, err)
	}
	hour, minute, second, nano, err := parseTimeComponents(timePart)
	if err != nil {
		return ZonedTime{}, fmt.Errorf("cannot parse %q as zoned_time: %w", s, err)
	}
	return ZonedTime{Hour: hour, Minute: minute, Second: second, Nanosecond: nano, OffsetMinutes: offset}, nil
}

func parseLocalDateTime(s string) (LocalDateTime, error) {
	s = strings.Replace(s, "T", " ", 1)
	s = removeOffset(s)
	parts := strings.SplitN(s, " ", 2)
	if len(parts) != 2 {
		return LocalDateTime{}, fmt.Errorf("cannot parse %q as local_datetime (expected yyyy-MM-dd HH:mm:ss[.n])", s)
	}
	datePart := parts[0]
	timePart := parts[1]

	dt, err := time.Parse("2006-01-02", datePart)
	if err != nil {
		return LocalDateTime{}, fmt.Errorf("cannot parse date part %q: %w", datePart, err)
	}
	hour, minute, second, nano, err := parseTimeComponents(timePart)
	if err != nil {
		return LocalDateTime{}, fmt.Errorf("cannot parse time part %q: %w", timePart, err)
	}
	t := time.Date(dt.Year(), dt.Month(), dt.Day(), int(hour), int(minute), int(second), int(nano), time.UTC)
	return LocalDateTime{Time: t}, nil
}

func parseZonedDateTime(s string) (ZonedDateTime, error) {
	s = strings.Replace(s, "T", " ", 1)
	s = normalizeOffset(s)
	parts := strings.SplitN(s, " ", 2)
	if len(parts) != 2 {
		return ZonedDateTime{}, fmt.Errorf("cannot parse %q as zoned_datetime", s)
	}
	datePart := parts[0]
	timeAndOffset := parts[1]

	dt, err := time.Parse("2006-01-02", datePart)
	if err != nil {
		return ZonedDateTime{}, fmt.Errorf("cannot parse date part %q: %w", datePart, err)
	}

	timePart, offset, err := splitOffset(timeAndOffset)
	if err != nil {
		return ZonedDateTime{}, fmt.Errorf("cannot parse %q as zoned_datetime: %w", s, err)
	}
	hour, minute, second, nano, err := parseTimeComponents(timePart)
	if err != nil {
		return ZonedDateTime{}, fmt.Errorf("cannot parse time part %q: %w", timePart, err)
	}
	loc := time.FixedZone("", int(offset)*60)
	t := time.Date(dt.Year(), dt.Month(), dt.Day(), int(hour), int(minute), int(second), int(nano), loc)
	return ZonedDateTime{Time: t, OffsetMinutes: offset}, nil
}

func parseTimestamp(s string) (time.Time, error) {
	// Try epoch seconds first (pure numeric string)
	if v, err := strconv.ParseInt(s, 10, 64); err == nil && !strings.Contains(s, "-") {
		return time.Unix(v, 0).UTC(), nil
	}
	// Try as zoned datetime (with offset like +08:00 or Z), convert to UTC
	if hasOffset(s) || strings.HasSuffix(s, "Z") {
		zdt, err := parseZonedDateTime(s)
		if err == nil {
			return zdt.Time.UTC(), nil
		}
	}
	// Parse as local datetime string (no offset)
	ldt, err := parseLocalDateTime(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse %q as timestamp: %w", s, err)
	}
	return ldt.Time.UTC(), nil
}

// hasOffset checks if a datetime string contains a timezone offset.
func hasOffset(s string) bool {
	for i := len(s) - 1; i > 8; i-- {
		if s[i] == '+' || s[i] == '-' {
			if len(s)-i >= 5 {
				return true
			}
		}
	}
	return false
}

func parseYearToMonth(s string) (YearToMonth, error) {
	negative := false
	p := s
	if strings.HasPrefix(p, "-") {
		negative = true
		p = p[1:]
	}
	if !strings.HasPrefix(p, "P") {
		return YearToMonth{}, fmt.Errorf("cannot parse %q as year_to_month (expected ISO-8601 like P2Y5M)", s)
	}
	p = p[1:] // remove P

	// Remove T part if present (shouldn't be, but be lenient)
	if idx := strings.Index(p, "T"); idx >= 0 {
		p = p[:idx]
	}

	var years, months int32
	var num string
	for _, c := range p {
		if c >= '0' && c <= '9' {
			num += string(c)
		} else if c == 'Y' {
			v, err := strconv.Atoi(num)
			if err != nil {
				return YearToMonth{}, fmt.Errorf("cannot parse %q as year_to_month: invalid year value", s)
			}
			years = int32(v)
			num = ""
		} else if c == 'M' {
			v, err := strconv.Atoi(num)
			if err != nil {
				return YearToMonth{}, fmt.Errorf("cannot parse %q as year_to_month: invalid month value", s)
			}
			months = int32(v)
			num = ""
		}
	}

	total := years*12 + months
	if negative {
		total = -total
	}
	return YearToMonth{Months: total}, nil
}

func parseDayToSecond(s string) (DayToSecond, error) {
	negative := false
	p := s
	if strings.HasPrefix(p, "-") {
		negative = true
		p = p[1:]
	}
	if !strings.HasPrefix(p, "P") {
		return DayToSecond{}, fmt.Errorf("cannot parse %q as day_to_second (expected ISO-8601 like P3DT4H)", s)
	}
	p = p[1:] // remove P

	var totalSeconds uint64
	var totalNanos uint32

	// Split into date part and time part at T
	datePart := p
	timePart := ""
	if idx := strings.Index(p, "T"); idx >= 0 {
		datePart = p[:idx]
		timePart = p[idx+1:]
	}

	// Parse date part (days only)
	if datePart != "" {
		var num string
		for _, c := range datePart {
			if c >= '0' && c <= '9' {
				num += string(c)
			} else if c == 'D' {
				v, err := strconv.ParseUint(num, 10, 64)
				if err != nil {
					return DayToSecond{}, fmt.Errorf("cannot parse %q: invalid day value", s)
				}
				totalSeconds += v * 86400
				num = ""
			}
		}
	}

	// Parse time part (hours, minutes, seconds)
	if timePart != "" {
		var num string
		for _, c := range timePart {
			if c >= '0' && c <= '9' || c == '.' {
				num += string(c)
			} else if c == 'H' {
				v, err := strconv.ParseUint(num, 10, 64)
				if err != nil {
					return DayToSecond{}, fmt.Errorf("cannot parse %q: invalid hour value", s)
				}
				totalSeconds += v * 3600
				num = ""
			} else if c == 'M' {
				v, err := strconv.ParseUint(num, 10, 64)
				if err != nil {
					return DayToSecond{}, fmt.Errorf("cannot parse %q: invalid minute value", s)
				}
				totalSeconds += v * 60
				num = ""
			} else if c == 'S' {
				if strings.Contains(num, ".") {
					parts := strings.SplitN(num, ".", 2)
					sec, err := strconv.ParseUint(parts[0], 10, 64)
					if err != nil {
						return DayToSecond{}, fmt.Errorf("cannot parse %q: invalid second value", s)
					}
					totalSeconds += sec
					// Parse fractional seconds as nanoseconds
					frac := parts[1]
					for len(frac) < 9 {
						frac += "0"
					}
					frac = frac[:9]
					n, _ := strconv.ParseUint(frac, 10, 32)
					totalNanos = uint32(n)
				} else {
					v, err := strconv.ParseUint(num, 10, 64)
					if err != nil {
						return DayToSecond{}, fmt.Errorf("cannot parse %q: invalid second value", s)
					}
					totalSeconds += v
				}
				num = ""
			}
		}
	}

	signedSecs := int64(totalSeconds)
	if negative {
		signedSecs = -signedSecs
	}
	return DayToSecond{Seconds: signedSecs, Nanoseconds: totalNanos}, nil
}

// parseTimeComponents parses "HH:mm:ss" or "HH:mm:ss.nnnnnnnnn"
func parseTimeComponents(s string) (hour, minute, second uint8, nanosecond uint32, err error) {
	// Split off nanoseconds
	mainPart := s
	var nanoStr string
	if idx := strings.Index(s, "."); idx >= 0 {
		mainPart = s[:idx]
		nanoStr = s[idx+1:]
	}

	parts := strings.Split(mainPart, ":")
	if len(parts) < 2 {
		return 0, 0, 0, 0, fmt.Errorf("expected at least HH:mm, got %q", s)
	}

	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, 0, 0, fmt.Errorf("invalid hour in %q", s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, 0, 0, 0, fmt.Errorf("invalid minute in %q", s)
	}

	var sec int
	if len(parts) >= 3 {
		sec, err = strconv.Atoi(parts[2])
		if err != nil || sec < 0 || sec > 59 {
			return 0, 0, 0, 0, fmt.Errorf("invalid second in %q", s)
		}
	}

	var nano uint32
	if nanoStr != "" {
		for len(nanoStr) < 9 {
			nanoStr += "0"
		}
		nanoStr = nanoStr[:9]
		n, err := strconv.ParseUint(nanoStr, 10, 32)
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid nanoseconds in %q", s)
		}
		nano = uint32(n)
	}

	return uint8(h), uint8(m), uint8(sec), nano, nil
}

// splitOffset splits "14:30:00+08:00" into ("14:30:00", 480).
// Supports +HH:MM, -HH:MM, +HHMM, -HHMM formats.
func splitOffset(s string) (timePart string, offsetMinutes int16, err error) {
	// Find the last + or - that indicates offset (skip the first char in case of negative time)
	idx := -1
	for i := len(s) - 1; i > 0; i-- {
		if s[i] == '+' || s[i] == '-' {
			// Make sure this is the offset separator, not part of time
			// Offset is at least 4 chars: +HH:MM or +HHMM
			remaining := s[i:]
			if len(remaining) >= 5 {
				idx = i
				break
			}
		}
	}
	if idx < 0 {
		return "", 0, fmt.Errorf("no timezone offset found in %q", s)
	}

	timePart = s[:idx]
	offsetStr := s[idx:]

	sign := int16(1)
	if offsetStr[0] == '-' {
		sign = -1
	}
	offsetStr = offsetStr[1:]
	offsetStr = strings.Replace(offsetStr, ":", "", 1)

	if len(offsetStr) != 4 {
		return "", 0, fmt.Errorf("invalid offset format in %q", s)
	}
	h, err := strconv.Atoi(offsetStr[:2])
	if err != nil {
		return "", 0, fmt.Errorf("invalid offset hours in %q", s)
	}
	m, err := strconv.Atoi(offsetStr[2:4])
	if err != nil {
		return "", 0, fmt.Errorf("invalid offset minutes in %q", s)
	}
	offsetMinutes = sign * int16(h*60+m)

	return timePart, offsetMinutes, nil
}

// =============================================================================
// Spatial / Vector / Blob parse helpers
// =============================================================================

// parseKeyedFloats extracts the "{k1: v1, k2: v2, ...}" body of a value like
// `point({latitude: 30.5, longitude: 114.3})` into a lower-cased key->float
// map. Keys are matched case-insensitively; whitespace is tolerated.
func parseKeyedFloats(s string) (map[string]float64, error) {
	l := strings.IndexByte(s, '{')
	r := strings.LastIndexByte(s, '}')
	if l < 0 || r < 0 || r < l {
		return nil, fmt.Errorf("expected {key: value, ...}")
	}
	out := make(map[string]float64)
	for _, pair := range strings.Split(s[l+1:r], ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("expected key: value, got %q", pair)
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		v, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number for %q: %w", key, err)
		}
		out[key] = v
	}
	return out, nil
}

// parseFloatList parses a comma-separated list of numbers, tolerating a single
// wrapping pair of (), [] or no brackets at all: "1,2,3", "(1,2,3)", "[1,2,3]".
// An empty payload (e.g. "[]") returns an empty, non-nil slice.
func parseFloatList(s string) ([]float64, error) {
	s = strings.TrimSpace(s)
	for _, pair := range [][2]byte{{'(', ')'}, {'[', ']'}, {'{', '}'}} {
		if len(s) >= 2 && s[0] == pair[0] && s[len(s)-1] == pair[1] {
			s = s[1 : len(s)-1]
			break
		}
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return []float64{}, nil
	}
	parts := strings.Split(s, ",")
	out := make([]float64, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q: %w", strings.TrimSpace(p), err)
		}
		out = append(out, v)
	}
	return out, nil
}

// parsePoint parses a 2D geographic point. Accepts the canonical
// `point({latitude: 30.5, longitude: 114.3})` form (keys, any order), the
// lenient positional form `30.5,114.3` / `(30.5,114.3)` (lat,lon), and the
// OGC/PostGIS WKT form `POINT(114.3 30.5)` — standard WKT order is
// `POINT(<longitude> <latitude>)`, i.e. lon FIRST (the opposite of the
// lenient comma form).
func parsePoint(s string) (Point, error) {
	if strings.IndexByte(s, '{') >= 0 {
		m, err := parseKeyedFloats(s)
		if err != nil {
			return Point{}, fmt.Errorf("cannot parse %q as point: %w", s, err)
		}
		lat, okLat := m["latitude"]
		lon, okLon := m["longitude"]
		if !okLat || !okLon {
			return Point{}, fmt.Errorf("cannot parse %q as point: expected latitude and longitude keys", s)
		}
		return newPoint(lat, lon, s)
	}
	if nums, isWKT, err := parseWKTPoint(s); isWKT {
		if err != nil {
			return Point{}, fmt.Errorf("cannot parse %q as point: %w", s, err)
		}
		if len(nums) != 2 {
			return Point{}, fmt.Errorf("cannot parse %q as point: WKT POINT expects 2 values (lon lat), got %d", s, len(nums))
		}
		// OGC WKT is POINT(lon lat) — longitude first.
		return newPoint(nums[1], nums[0], s)
	}
	nums, err := parseFloatList(s)
	if err != nil {
		return Point{}, fmt.Errorf("cannot parse %q as point (expected point({latitude:..,longitude:..}), lat,lon, or POINT(lon lat)): %w", s, err)
	}
	if len(nums) != 2 {
		return Point{}, fmt.Errorf("cannot parse %q as point: expected 2 values (lat,lon), got %d", s, len(nums))
	}
	return newPoint(nums[0], nums[1], s)
}

// parseWKTPoint detects an OGC WKT-style "POINT(a b ...)" literal (case
// insensitive, optional Z/M dimension tag) and returns the contained floats.
// Separators may be spaces and/or commas. isWKT reports whether s looked like
// a WKT POINT literal at all; when true but the body is malformed, err is set.
// Returns the raw floats in source order; the caller assigns meaning — the 2D
// geographic Point reads them as OGC lon,lat (longitude first), the 3D Point3D
// as cartesian x,y,z.
func parseWKTPoint(s string) (nums []float64, isWKT bool, err error) {
	t := strings.TrimSpace(s)
	if len(t) < 5 || !strings.EqualFold(t[:5], "POINT") {
		return nil, false, nil
	}
	t = strings.TrimSpace(t[5:])
	// Skip an optional dimension tag like "Z", "M" or "ZM".
	for len(t) > 0 && (t[0] == 'z' || t[0] == 'Z' || t[0] == 'm' || t[0] == 'M') {
		t = strings.TrimSpace(t[1:])
	}
	if len(t) < 2 || t[0] != '(' || t[len(t)-1] != ')' {
		return nil, false, nil
	}
	inner := strings.TrimSpace(t[1 : len(t)-1])
	if inner == "" {
		return []float64{}, true, nil
	}
	fields := strings.FieldsFunc(inner, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t'
	})
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		v, perr := strconv.ParseFloat(f, 64)
		if perr != nil {
			return nil, true, fmt.Errorf("invalid number %q in WKT POINT: %w", f, perr)
		}
		out = append(out, v)
	}
	return out, true, nil
}

func newPoint(lat, lon float64, s string) (Point, error) {
	if lat < -90 || lat > 90 {
		return Point{}, fmt.Errorf("cannot parse %q as point: latitude %v out of range [-90,90]", s, lat)
	}
	if lon < -180 || lon > 180 {
		return Point{}, fmt.Errorf("cannot parse %q as point: longitude %v out of range [-180,180]", s, lon)
	}
	return Point{Latitude: lat, Longitude: lon}, nil
}

// parsePoint3D parses a 3D cartesian point. Accepts the canonical
// `point({x: 1, y: 2, z: 3})` form (keys, any order), the lenient positional
// form `1,2,3` / `(1,2,3)`, and the WKT form `POINT(1 2 3)` / `POINT Z(1 2 3)`.
func parsePoint3D(s string) (Point3D, error) {
	if strings.IndexByte(s, '{') >= 0 {
		m, err := parseKeyedFloats(s)
		if err != nil {
			return Point3D{}, fmt.Errorf("cannot parse %q as point3d: %w", s, err)
		}
		x, okX := m["x"]
		y, okY := m["y"]
		z, okZ := m["z"]
		if !okX || !okY || !okZ {
			return Point3D{}, fmt.Errorf("cannot parse %q as point3d: expected x, y and z keys", s)
		}
		return Point3D{X: x, Y: y, Z: z}, nil
	}
	if nums, isWKT, err := parseWKTPoint(s); isWKT {
		if err != nil {
			return Point3D{}, fmt.Errorf("cannot parse %q as point3d: %w", s, err)
		}
		if len(nums) != 3 {
			return Point3D{}, fmt.Errorf("cannot parse %q as point3d: WKT POINT expects 3 values (x y z), got %d", s, len(nums))
		}
		return Point3D{X: nums[0], Y: nums[1], Z: nums[2]}, nil
	}
	nums, err := parseFloatList(s)
	if err != nil {
		return Point3D{}, fmt.Errorf("cannot parse %q as point3d (expected point({x:..,y:..,z:..}), x,y,z, or POINT(x y z)): %w", s, err)
	}
	if len(nums) != 3 {
		return Point3D{}, fmt.Errorf("cannot parse %q as point3d: expected 3 values (x,y,z), got %d", s, len(nums))
	}
	return Point3D{X: nums[0], Y: nums[1], Z: nums[2]}, nil
}

// parseVector parses a float32 vector. Accepts `[0.1,0.2,0.3]` and the
// bracket-less `0.1,0.2,0.3`; `[]` yields an empty (non-nil) vector.
func parseVector(s string) (Vector, error) {
	nums, err := parseFloatList(s)
	if err != nil {
		return Vector{}, fmt.Errorf("cannot parse %q as vector (expected [n,n,...]): %w", s, err)
	}
	vals := make([]float32, len(nums))
	for i, n := range nums {
		vals[i] = float32(n)
	}
	return Vector{Values: vals}, nil
}

// parseBlob parses binary data. Default is Base64 (StdEncoding); a `0x`/`0X`
// prefix selects hex. The encoding must be documented for the data source.
func parseBlob(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		b, err := hex.DecodeString(s[2:])
		if err != nil {
			return nil, fmt.Errorf("cannot parse %q as blob (invalid hex): %w", s, err)
		}
		return b, nil
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q as blob (invalid base64): %w", s, err)
	}
	return b, nil
}
