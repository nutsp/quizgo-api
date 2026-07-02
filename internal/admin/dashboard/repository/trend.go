package repository

import "time"

var thaiShortMonths = []string{
	"", "ม.ค.", "ก.พ.", "มี.ค.", "เม.ย.", "พ.ค.", "มิ.ย.",
	"ก.ค.", "ส.ค.", "ก.ย.", "ต.ค.", "พ.ย.", "ธ.ค.",
}

func formatThaiShortDateLabel(t time.Time) string {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Format("2") +
		" " + thaiShortMonths[int(t.Month())]
}

func trendStartDate(days int, loc *time.Location) time.Time {
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return today.AddDate(0, 0, -(days - 1))
}

func FillDailyTrend(days int, rows []AdminTrendPoint, loc *time.Location) []AdminTrendPoint {
	totals := make(map[string]int64, len(rows))
	for _, row := range rows {
		totals[row.Date] = row.Total
	}

	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	start := today.AddDate(0, 0, -(days - 1))

	result := make([]AdminTrendPoint, 0, days)
	for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		result = append(result, AdminTrendPoint{
			Date:  dateStr,
			Label: formatThaiShortDateLabel(d),
			Total: totals[dateStr],
		})
	}
	return result
}
