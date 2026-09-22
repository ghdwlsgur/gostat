package cmd

import (
	"reflect"
	"testing"

	"github.com/ghdwlsgur/gostat/internal"
	ui "github.com/gizak/termui/v3"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name         string
		arg          string
		wantProtocol string
		wantRest     string
		wantErr      bool
	}{
		{
			name:         "https url",
			arg:          "https://www.naver.com/index.html",
			wantProtocol: "https",
			wantRest:     "www.naver.com/index.html",
		},
		{
			name:         "http url",
			arg:          "http://example.com",
			wantProtocol: "http",
			wantRest:     "example.com",
		},
		{
			// This used to slip past the check and then panic on splitData[1].
			name:    "a bare domain is rejected, not panicked on",
			arg:     "www.naver.com",
			wantErr: true,
		},
		{
			name:    "other protocols are rejected",
			arg:     "ftp://example.com",
			wantErr: true,
		},
		{
			name:    "a protocol with no host is rejected",
			arg:     "https://",
			wantErr: true,
		},
		{
			name:    "a path with no host is rejected",
			arg:     "https:///index.html",
			wantErr: true,
		},
		{
			name:    "an empty argument is rejected",
			arg:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protocol, rest, err := parseURL(tt.arg)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseURL(%q) = (%q, %q, nil), want an error", tt.arg, protocol, rest)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseURL(%q): %v", tt.arg, err)
			}
			if protocol != tt.wantProtocol {
				t.Errorf("protocol = %q, want %q", protocol, tt.wantProtocol)
			}
			if rest != tt.wantRest {
				t.Errorf("rest = %q, want %q", rest, tt.wantRest)
			}
		})
	}
}

func TestResolvePort(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		flagPort int
		want     int
	}{
		{"http falls back to 80", "http", 0, internal.DefaultHTTPPort},
		{"https falls back to 443", "https", 0, internal.DefaultHTTPSPort},
		{"an explicit port wins over http", "http", 8080, 8080},
		{"an explicit port wins over https", "https", 8443, 8443},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolvePort(tt.protocol, tt.flagPort); got != tt.want {
				t.Errorf("resolvePort(%q, %d) = %d, want %d", tt.protocol, tt.flagPort, got, tt.want)
			}
		})
	}
}

func TestUniqueBox(t *testing.T) {
	box := &uniqueBox{}

	box.Add("200")
	box.Add("200")
	box.Add("404")

	if box.Length() != 2 {
		t.Errorf("Length() = %d, want 2 after adding a duplicate", box.Length())
	}
	if !box.Contain("404") {
		t.Error(`Contain("404") = false`)
	}
	if box.Contain("500") {
		t.Error(`Contain("500") = true`)
	}

	box.Remove("200")
	if box.Contain("200") {
		t.Error(`Remove("200") left the value behind`)
	}
	if box.Length() != 1 {
		t.Errorf("Length() = %d, want 1 after a removal", box.Length())
	}

	box.Remove("nothing-like-this")
	if box.Length() != 1 {
		t.Errorf("removing an absent value changed Length() to %d", box.Length())
	}
}

func TestUniqueBoxGetReturnsACopy(t *testing.T) {
	box := &uniqueBox{}
	box.Add("200")

	got := box.Get()
	got[0] = "tampered"

	if box.Get()[0] != "200" {
		t.Error("Get() handed out the backing array; the history table can corrupt the box")
	}
}

func TestDynamicStatusCodeColor(t *testing.T) {
	tests := []struct {
		statusCode int
		want       []ui.Color
	}{
		{200, []ui.Color{2}},
		{301, []ui.Color{4}},
		{404, []ui.Color{3}},
		{503, []ui.Color{1}},
	}

	for _, tt := range tests {
		if got := dynamicStatusCodeColor(tt.statusCode, nil); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("dynamicStatusCodeColor(%d) = %v, want %v", tt.statusCode, got, tt.want)
		}
	}

	// An unclassified code leaves the previous colour in place.
	previous := []ui.Color{7}
	if got := dynamicStatusCodeColor(100, previous); !reflect.DeepEqual(got, previous) {
		t.Errorf("dynamicStatusCodeColor(100) = %v, want the colour untouched %v", got, previous)
	}
}

// A CDN domain routinely answers with more than nine A records; the chart used
// to be sized to a fixed nine and indexed by edge, so the tenth edge panicked.
func TestCreateEdgeChartHasOneSlotPerEdge(t *testing.T) {
	ips := make([]string, 12)
	for i := range ips {
		ips[i] = "10.0.0." + string(rune('a'+i))
	}

	charts := createEdgeChart("example.com", ips)
	if len(charts) != len(ips) {
		t.Fatalf("got %d charts, want %d", len(charts), len(ips))
	}

	for ip, chart := range charts {
		if len(chart.Data) != len(ips) {
			t.Fatalf("chart for %s has %d data slots, want %d", ip, len(chart.Data), len(ips))
		}
	}
}

func TestCreateResponseTableHasAColumnPerEdge(t *testing.T) {
	ips := []string{"1.1.1.1", "2.2.2.2"}
	table := createResponseTable(ips)

	if len(table.Rows) != 15 {
		t.Fatalf("got %d rows, want 15", len(table.Rows))
	}

	wantHeader := []string{"IP", "1.1.1.1", "2.2.2.2"}
	if !reflect.DeepEqual(table.Rows[0], wantHeader) {
		t.Errorf("header = %v, want %v", table.Rows[0], wantHeader)
	}

	for i, row := range table.Rows {
		if len(row) != len(ips)+1 {
			t.Errorf("row %d has %d cells, want %d", i, len(row), len(ips)+1)
		}
	}
}
