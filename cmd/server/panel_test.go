package main

import (
	"reflect"
	"testing"
	"time"

	"nudge/internal/pair"
)

var panelNow = time.Date(2026, 11, 23, 20, 0, 0, 0, time.Local)

func TestPanelHeader(t *testing.T) {
	m := panelModelFrom("ZEPHYRUS", "6B86-B273-FF34", nil, nil, panelNow)
	if m.Host != "ZEPHYRUS" || m.Code != "6B86-B273-FF34" {
		t.Fatalf("header = %q / %q", m.Host, m.Code)
	}
	if m := panelModelFrom("", "", nil, nil, panelNow); m.Host != "Nudge" || m.Code != "" {
		t.Fatalf("fallbacks = %q / %q", m.Host, m.Code)
	}
	if m := panelModelFrom("pc\u202e\n", "", nil, nil, panelNow); m.Host != "pc" {
		t.Fatalf("sanitized host = %q", m.Host)
	}
}

func TestPanelEmptyLists(t *testing.T) {
	m := panelModelFrom("pc", "", nil, nil, panelNow)
	if len(m.Requests) != 0 || len(m.Devices) != 0 {
		t.Fatalf("rows = %v / %v, want none", m.Requests, m.Devices)
	}
}

func TestPanelRequestsOldestFirstWithAge(t *testing.T) {
	reqs := []pair.RequestInfo{
		{RequestID: "r2", Name: "Galaxy", IP: "192.168.0.20", RequestedAt: panelNow.Add(-3 * time.Minute)},
		{RequestID: "r1", Name: "Pixel\u202e", IP: "192.168.0.13", RequestedAt: panelNow.Add(-2 * time.Hour)},
		{RequestID: "r3", Name: "", IP: "", RequestedAt: panelNow.Add(-10 * time.Second)},
	}
	got := panelModelFrom("pc", "", nil, reqs, panelNow).Requests
	want := []panelRequest{
		{ID: "r1", Title: "Pixel quiere emparejarse", Detail: "192.168.0.13 · hace 2 h. ¿El teléfono muestra el mismo código?"},
		{ID: "r2", Title: "Galaxy quiere emparejarse", Detail: "192.168.0.20 · hace 3 min. ¿El teléfono muestra el mismo código?"},
		{ID: "r3", Title: "(sin nombre) quiere emparejarse", Detail: "(IP desconocida) · recién. ¿El teléfono muestra el mismo código?"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("requests = %q, want %q", got, want)
	}
}

func TestPanelDevicesSortedWithPairingDate(t *testing.T) {
	devs := []pair.DeviceInfo{
		{ID: "b", Name: "pixel", IP: "192.168.0.13", FirstApproved: time.Date(2026, 11, 19, 18, 0, 0, 0, time.Local)},
		{ID: "a", Name: "Galaxy", IP: ""},
	}
	got := panelModelFrom("pc", "", devs, nil, panelNow).Devices
	want := []panelDevice{
		{ID: "a", Name: "Galaxy", IP: "(IP desconocida)", Date: ""},
		{ID: "b", Name: "pixel", IP: "192.168.0.13", Date: "emparejado el 19/11/2026"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("devices = %q, want %q", got, want)
	}
}

func TestPanelMore(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{panelMoreRequests(0), ""},
		{panelMoreRequests(1), "y 1 solicitud más"},
		{panelMoreRequests(3), "y 3 solicitudes más"},
		{panelMoreDevices(0), ""},
		{panelMoreDevices(1), "y 1 teléfono más"},
		{panelMoreDevices(2), "y 2 teléfonos más"},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("case %d = %q, want %q", i, c.got, c.want)
		}
	}
}

func TestPanelAgo(t *testing.T) {
	cases := map[time.Duration]string{
		-time.Minute:     "recién", // clock skew
		59 * time.Second: "recién",
		time.Minute:      "hace 1 min",
		59 * time.Minute: "hace 59 min",
		90 * time.Minute: "hace 1 h",
		49 * time.Hour:   "hace 2 d",
	}
	for d, want := range cases {
		if got := panelAgo(panelNow.Add(-d), panelNow); got != want {
			t.Errorf("panelAgo(%v) = %q, want %q", d, got, want)
		}
	}
}
