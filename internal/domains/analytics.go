package domains

// FP-P3-052 — first-party lightweight website analytics (P3 §52, observe):
// requests, response codes, top error routes, traffic trends and referrers
// where privacy policy allows. NO third-party scripts, NO invasive tracking.
// Retention and privacy controls are first-class (opt-out, purge horizon).

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// AccessLogLine is one parsed request from the host access log (self-hosted,
// first-party — the data never leaves the installation).
type AccessLogLine struct {
	TS           time.Time
	Path         string
	Status       int
	Bytes        int64
	ReferrerHost string // host only; full referrer URLs are not retained
}

// SitePrivacySettings are the privacy controls for site analytics.
type SitePrivacySettings struct {
	Enabled         bool // privacy opt-out: analytics disabled entirely
	RecordReferrers bool // policy-gated referrer recording
	RetentionDays   int  // purge horizon
}

// DefaultSitePrivacy is the shipped default: enabled, referrers ON (host
// only), 90-day retention. Operators configure per organization/plan scope.
func DefaultSitePrivacy() SitePrivacySettings {
	return SitePrivacySettings{Enabled: true, RecordReferrers: true, RetentionDays: 90}
}

// ValidateSitePrivacy enforces bounds (retention 7..3650, no negative).
func ValidateSitePrivacy(p SitePrivacySettings) error {
	if p.RetentionDays < 7 || p.RetentionDays > 3650 {
		return fmt.Errorf("retention must be between 7 and 3650 days")
	}
	return nil
}

// ParseAccessLogLine parses the platform access-log format:
//
//	<unix-ts> "<method> <path> <proto>" <status> <bytes> "<referrer>"
func ParseAccessLogLine(line string) (AccessLogLine, error) {
	var l AccessLogLine
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		return l, fmt.Errorf("malformed line")
	}
	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return l, fmt.Errorf("bad timestamp")
	}
	l.TS = time.Unix(ts, 0).UTC()
	rest := parts[1]
	// request part
	q1 := strings.Index(rest, `"`)
	if q1 < 0 {
		return l, fmt.Errorf("bad request field")
	}
	rest2 := rest[q1+1:]
	q2 := strings.Index(rest2, `"`)
	if q2 < 0 {
		return l, fmt.Errorf("bad request close")
	}
	req := rest2[:q2]
	fields := strings.Fields(req)
	if len(fields) >= 2 {
		l.Path = fields[1]
	}
	rest3 := rest2[q2+1:]
	f := strings.Fields(strings.TrimSpace(rest3))
	if len(f) < 2 {
		return l, fmt.Errorf("bad status/bytes")
	}
	status, err := strconv.Atoi(f[0])
	if err != nil {
		return l, fmt.Errorf("bad status")
	}
	l.Status = status
	bytes, _ := strconv.ParseInt(f[1], 10, 64)
	l.Bytes = bytes
	// referrer (optional, host only)
	if idx := strings.Index(rest3, `"`); idx >= 0 {
		tail := rest3[idx+1:]
		if end := strings.Index(tail, `"`); end >= 0 {
			raw := tail[:end]
			if raw != "" && raw != "-" {
				l.ReferrerHost = referrerHostOnly(raw)
			}
		}
	}
	return l, nil
}

// referrerHostOnly reduces a referrer to its host — full URLs are never
// retained (privacy control).
func referrerHostOnly(raw string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "@"); i >= 0 { // strip userinfo — host only
		s = s[i+1:]
	}
	return strings.ToLower(s)
}

// SiteAnalytics is the aggregated view served to the website owner.
type SiteAnalytics struct {
	TotalRequests  int64            `json:"total_requests"`
	StatusCounts   map[string]int64 `json:"status_counts"`
	TopErrorRoutes []RouteCount     `json:"top_error_routes"`
	Trend          []TrendPoint     `json:"trend"`
	TopReferrers   []RouteCount     `json:"top_referrers"`
}

// RouteCount is a path/host with its hit count.
type RouteCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// TrendPoint is a bucketed traffic count.
type TrendPoint struct {
	Bucket string `json:"bucket"` // e.g. 2026-11-14T05:00Z
	Count  int64  `json:"count"`
}

// AggregateAnalytics computes the analytics view from parsed lines.
// Privacy settings gate referrers entirely; error routes = 4xx/5xx paths.
func AggregateAnalytics(lines []AccessLogLine, privacy SitePrivacySettings) SiteAnalytics {
	out := SiteAnalytics{StatusCounts: map[string]int64{}}
	errRoutes := map[string]int64{}
	refs := map[string]int64{}
	buckets := map[string]int64{}
	for _, l := range lines {
		out.TotalRequests++
		out.StatusCounts[strconv.Itoa(l.Status/100)+"xx"]++
		if l.Status >= 400 {
			errRoutes[l.Path]++
		}
		if privacy.RecordReferrers && l.ReferrerHost != "" {
			refs[l.ReferrerHost]++
		}
		b := l.TS.Truncate(time.Hour).Format("2006-01-02T15:04Z")
		buckets[b]++
	}
	out.TopErrorRoutes = topN(errRoutes, 5)
	out.Trend = trendSorted(buckets)
	out.TopReferrers = topN(refs, 5)
	return out
}

func topN(m map[string]int64, n int) []RouteCount {
	list := make([]RouteCount, 0, len(m))
	for k, v := range m {
		list = append(list, RouteCount{k, v})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count == list[j].Count {
			return list[i].Name < list[j].Name
		}
		return list[i].Count > list[j].Count
	})
	if len(list) > n {
		list = list[:n]
	}
	return list
}

func trendSorted(m map[string]int64) []TrendPoint {
	list := make([]TrendPoint, 0, len(m))
	for k, v := range m {
		list = append(list, TrendPoint{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Bucket < list[j].Bucket })
	return list
}

// RetainHits filters parsed lines against the privacy retention horizon and
// the enabled flag — the purge horizon is enforced BEFORE persistence.
func RetainHits(lines []AccessLogLine, privacy SitePrivacySettings, now time.Time) []AccessLogLine {
	if !privacy.Enabled {
		return nil
	}
	horizon := now.AddDate(0, 0, -privacy.RetentionDays)
	kept := lines[:0]
	for _, l := range lines {
		if !l.TS.Before(horizon) {
			kept = append(kept, l)
		}
	}
	return kept
}
