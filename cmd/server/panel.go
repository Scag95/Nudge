package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"nudge/internal/pair"
)

// Content of the Windows panel window (panel_windows.go). Built here,
// platform-neutral, so it is unit tested everywhere. LAN-supplied names and
// IPs are sanitized and truncated exactly like the pairing dialog.

const maxPanelHost = 40

// panelRequest is a pending pairing request as the banner shows it.
type panelRequest struct {
	ID     string
	Title  string // "<name> quiere emparejarse"
	Detail string // "<ip> · <age>. ¿El teléfono muestra el mismo código?"
}

// panelDevice is a paired phone row.
type panelDevice struct {
	ID   string
	Name string
	IP   string
	Date string // "emparejado el DD/MM/AAAA", empty if unknown
}

type panelModel struct {
	Host     string         // status pill text (host name only)
	Code     string         // empty until the certificate code is known
	Requests []panelRequest // pending pairing requests, oldest first
	Devices  []panelDevice  // approved phones, by name
}

func panelModelFrom(host, code string, devices []pair.DeviceInfo, requests []pair.RequestInfo, now time.Time) panelModel {
	if host = sanitizeDialogField(host, maxPanelHost); host == "" {
		host = "Nudge"
	}
	m := panelModel{Host: host, Code: code}

	reqs := append([]pair.RequestInfo(nil), requests...)
	sort.SliceStable(reqs, func(i, j int) bool {
		if !reqs[i].RequestedAt.Equal(reqs[j].RequestedAt) {
			return reqs[i].RequestedAt.Before(reqs[j].RequestedAt)
		}
		return reqs[i].RequestID < reqs[j].RequestID
	})
	for _, r := range reqs {
		m.Requests = append(m.Requests, panelRequest{
			ID:     r.RequestID,
			Title:  panelName(r.Name) + " quiere emparejarse",
			Detail: panelIP(r.IP) + " · " + panelAgo(r.RequestedAt, now) + ". ¿El teléfono muestra el mismo código?",
		})
	}

	devs := append([]pair.DeviceInfo(nil), devices...)
	sort.SliceStable(devs, func(i, j int) bool {
		a, b := strings.ToLower(panelName(devs[i].Name)), strings.ToLower(panelName(devs[j].Name))
		if a != b {
			return a < b
		}
		if devs[i].IP != devs[j].IP {
			return devs[i].IP < devs[j].IP
		}
		return devs[i].ID < devs[j].ID
	})
	for _, d := range devs {
		row := panelDevice{ID: d.ID, Name: panelName(d.Name), IP: panelIP(d.IP)}
		if !d.FirstApproved.IsZero() {
			row.Date = "emparejado el " + d.FirstApproved.Local().Format("02/01/2006")
		}
		m.Devices = append(m.Devices, row)
	}
	return m
}

func panelName(name string) string {
	if name = sanitizeDialogField(name, maxDialogName); name == "" {
		return trayNoName
	}
	return name
}

func panelIP(ip string) string {
	if ip = sanitizeDialogField(ip, maxDialogIP); ip == "" {
		return trayNoIP
	}
	return ip
}

// panelMoreRequests notes the pending requests not shown in the banner.
func panelMoreRequests(n int) string {
	switch {
	case n <= 0:
		return ""
	case n == 1:
		return "y 1 solicitud más"
	default:
		return fmt.Sprintf("y %d solicitudes más", n)
	}
}

// panelMoreDevices notes the paired phones beyond the visible rows.
func panelMoreDevices(n int) string {
	switch {
	case n <= 0:
		return ""
	case n == 1:
		return "y 1 teléfono más"
	default:
		return fmt.Sprintf("y %d teléfonos más", n)
	}
}

// panelAgo is a short Spanish relative time ("recién", "hace 3 min").
func panelAgo(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "recién"
	case d < time.Hour:
		return fmt.Sprintf("hace %d min", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("hace %d h", int(d/time.Hour))
	default:
		return fmt.Sprintf("hace %d d", int(d/(24*time.Hour)))
	}
}
