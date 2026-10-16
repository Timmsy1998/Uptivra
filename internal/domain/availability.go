package domain

import "time"

// Availability uses UTC calendar days, including today's partial day. A sample
// represents at most its recorded cadence; longer gaps remain unmeasured.
type DailyAvailability struct {
	Date           string   `json:"date"`
	PeriodSeconds  float64  `json:"period_seconds"`
	CoveredSeconds float64  `json:"covered_seconds"`
	UpSeconds      float64  `json:"up_seconds"`
	MissingSeconds float64  `json:"missing_seconds"`
	Uptime         *float64 `json:"uptime"`
}
type Availability struct {
	MonitorID      string              `json:"monitor_id"`
	Days           int                 `json:"days"`
	From           time.Time           `json:"from"`
	To             time.Time           `json:"to"`
	RetentionDays  int                 `json:"retention_days"`
	PeriodSeconds  float64             `json:"period_seconds"`
	CoveredSeconds float64             `json:"covered_seconds"`
	UpSeconds      float64             `json:"up_seconds"`
	MissingSeconds float64             `json:"missing_seconds"`
	Uptime         *float64            `json:"uptime"`
	Daily          []DailyAvailability `json:"daily"`
	Incidents      []IncidentDuration  `json:"incidents"`
}
type IncidentDuration struct {
	ID              string     `json:"id"`
	OpenedAt        time.Time  `json:"opened_at"`
	ResolvedAt      *time.Time `json:"resolved_at"`
	DurationSeconds float64    `json:"duration_seconds"`
}

func ValidHistoryDays(days int) bool { return days == 7 || days == 30 || days == 90 }
func NewAvailability(id string, days, retention int, now time.Time) Availability {
	now = now.UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	from := midnight.AddDate(0, 0, 1-days)
	a := Availability{MonitorID: id, Days: days, From: from, To: now, RetentionDays: retention, Daily: []DailyAvailability{}, Incidents: []IncidentDuration{}}
	for i := 0; i < days; i++ {
		start := from.AddDate(0, 0, i)
		end := start.AddDate(0, 0, 1)
		if end.After(now) {
			end = now
		}
		a.Daily = append(a.Daily, DailyAvailability{Date: start.Format("2006-01-02"), PeriodSeconds: end.Sub(start).Seconds()})
	}
	return a
}

// AddCoverage accepts chronological, non-overlapping sample intervals.
func (a *Availability) AddCoverage(start, end time.Time, ok bool) {
	if start.Before(a.From) {
		start = a.From
	}
	if end.After(a.To) {
		end = a.To
	}
	for start.Before(end) {
		index := int(start.Sub(a.From) / (24 * time.Hour))
		dayEnd := a.From.AddDate(0, 0, index+1)
		if dayEnd.After(end) {
			dayEnd = end
		}
		seconds := dayEnd.Sub(start).Seconds()
		a.Daily[index].CoveredSeconds += seconds
		if ok {
			a.Daily[index].UpSeconds += seconds
		}
		start = dayEnd
	}
}
func (a *Availability) Finish() {
	for i := range a.Daily {
		d := &a.Daily[i]
		d.MissingSeconds = d.PeriodSeconds - d.CoveredSeconds
		if d.CoveredSeconds > 0 {
			v := 100 * d.UpSeconds / d.CoveredSeconds
			d.Uptime = &v
		}
		a.PeriodSeconds += d.PeriodSeconds
		a.CoveredSeconds += d.CoveredSeconds
		a.UpSeconds += d.UpSeconds
		a.MissingSeconds += d.MissingSeconds
	}
	if a.CoveredSeconds > 0 {
		v := 100 * a.UpSeconds / a.CoveredSeconds
		a.Uptime = &v
	}
}
