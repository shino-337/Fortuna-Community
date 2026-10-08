package enrich

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const kevDoc = `{"vulnerabilities":[
 {"cveID":"CVE-2024-3094","dateAdded":"2024-04-01","dueDate":"2024-04-22","knownRansomwareCampaignUse":"Unknown"},
 {"cveID":"cve-2021-44228","dateAdded":"2021-12-10","dueDate":"","knownRansomwareCampaignUse":"Known"},
 {"cveID":"CVE-2024-3094","dateAdded":"2024-05-01"},
 {"cveID":"NOT-A-CVE","dateAdded":"2024-01-01"},
 {"cveID":"CVE-2020-0001","dateAdded":"bad"}]}`

func TestParseKEV(t *testing.T) {
	got, err := ParseKEV(strings.NewReader(kevDoc))
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "CVE-2024-3094", got[0].CVEID)
	require.Equal(t, time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC), got[0].DateAdded)
	require.NotNil(t, got[0].DueDate)
	require.False(t, got[0].Ransomware)
	require.Equal(t, "CVE-2021-44228", got[1].CVEID)
	require.Nil(t, got[1].DueDate)
	require.True(t, got[1].Ransomware)

	_, err = ParseKEV(strings.NewReader(`{"vulnerabilities":[]}`))
	require.Error(t, err)
}

const epssCSV = "#model_version:v2025.03.14,score_date:2026-10-08T12:55:00Z\ncve,epss,percentile\nCVE-2024-3094,0.84,0.99\ncve-2021-44228,0.97,1.0\nCVE-2024-0001,1.5,0.1\nGHSA-x,0.1,0.1\n"

func TestParseEPSS(t *testing.T) {
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write([]byte(epssCSV))
	require.NoError(t, w.Close())

	for name, input := range map[string][]byte{"plain": []byte(epssCSV), "gzip": gz.Bytes()} {
		t.Run(name, func(t *testing.T) {
			scores, day, err := ParseEPSS(bytes.NewReader(input))
			require.NoError(t, err)
			require.Equal(t, time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), day)
			require.Equal(t, []EPSSScore{{"CVE-2024-3094", 0.84, 0.99}, {"CVE-2021-44228", 0.97, 1.0}}, scores)
		})
	}
	_, _, err := ParseEPSS(strings.NewReader("cve,score\nCVE-1,0.1\n"))
	require.Error(t, err)
}
