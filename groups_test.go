package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchPublicGroupRatiosParsesNumericGroups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", request.Method)
		}
		_, _ = writer.Write([]byte(`{"success":true,"message":"","data":{"VIP":{"desc":"会员","ratio":1.2},"经济":{"desc":"低价","ratio":0.1},"auto":{"desc":"自动","ratio":"自动"}}}`))
	}))
	defer server.Close()

	report, err := fetchPublicGroupRatios(server.Client(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if report.Endpoint != server.URL || len(report.Groups) != 2 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if report.Groups[0].Name != "经济" || report.Groups[0].Ratio != 0.1 {
		t.Fatalf("groups were not sorted by ratio: %#v", report.Groups)
	}
	if report.Groups[1].Description != "会员" || report.Groups[1].Ratio != 1.2 {
		t.Fatalf("unexpected group payload: %#v", report.Groups[1])
	}
}
