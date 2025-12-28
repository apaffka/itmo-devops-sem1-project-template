package prices

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

type ExportFilters struct {
	Start *time.Time
	End   *time.Time
	Min   *int64
	Max   *int64
}

func ParseExportFilters(q url.Values) (ExportFilters, error) {
	var f ExportFilters

	if v := q.Get("start"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return f, fmt.Errorf("invalid start (expected YYYY-MM-DD)")
		}
		f.Start = &t
	}
	if v := q.Get("end"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return f, fmt.Errorf("invalid end (expected YYYY-MM-DD)")
		}
		f.End = &t
	}
	if f.Start != nil && f.End != nil && f.Start.After(*f.End) {
		return f, fmt.Errorf("start must be <= end")
	}

	if v := q.Get("min"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return f, fmt.Errorf("invalid min (expected natural number > 0)")
		}
		f.Min = &n
	}
	if v := q.Get("max"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return f, fmt.Errorf("invalid max (expected natural number > 0)")
		}
		f.Max = &n
	}
	if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
		return f, fmt.Errorf("min must be <= max")
	}

	return f, nil
}
