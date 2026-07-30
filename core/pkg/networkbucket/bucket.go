package networkbucket

import "time"

const fiveMinSec = 5 * 60

// FloorBucket5MUTC returns the UTC start of the 5-minute wall-clock bucket containing t
// (aligned to Unix epoch multiples of 300 seconds).
func FloorBucket5MUTC(t time.Time) time.Time {
	u := t.UTC().Unix()
	return time.Unix(u-u%fiveMinSec, 0).UTC()
}
