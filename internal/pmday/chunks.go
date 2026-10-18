package pmday

import "time"

// DateRange is an inclusive calendar-day span (local dates as midnight local).
type DateRange struct {
	Start time.Time
	End   time.Time
}

// DateChunks splits [timeMin, timeMax] into inclusive date ranges of at most maxDays calendar days.
// Plan My Day agenda rejects ranges longer than 31 days.
func DateChunks(timeMin, timeMax time.Time, maxDays int) []DateRange {
	if maxDays < 1 {
		maxDays = 31
	}
	loc := timeMin.Location()
	if loc == nil {
		loc = time.Local
	}
	start := dateOnly(timeMin.In(loc))
	end := dateOnly(timeMax.In(loc))
	if end.Before(start) {
		return nil
	}
	var out []DateRange
	cur := start
	for !cur.After(end) {
		chunkEnd := cur.AddDate(0, 0, maxDays-1)
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		out = append(out, DateRange{Start: cur, End: chunkEnd})
		cur = chunkEnd.AddDate(0, 0, 1)
	}
	return out
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
