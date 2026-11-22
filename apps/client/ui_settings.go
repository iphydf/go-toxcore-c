package main

import (
	"errors"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// settingsPage is the settings part of the Me page.
type settingsPage struct {
	udp            widget.Bool
	ipv6           widget.Bool
	localDiscovery widget.Bool
	background     widget.Bool
	proxy          widget.Enum
	proxyHost      widget.Editor
	proxyPort      widget.Editor
	nodes          widget.Editor
	saveNetwork    widget.Clickable
	networkErr     string

	dark     widget.Bool
	textSize widget.Enum

	password       widget.Editor
	password2      widget.Editor
	setPassword    widget.Clickable
	removePassword confirmation
	passwordMsg    string
	export         widget.Clickable
	deleteProfile  confirmation
	deleteMsg      string
}

// textSizes are the text sizes by their keys for widget.Enum.
var textSizes = map[string]float32{"small": 0.875, "normal": 1, "large": 1.15, "larger": 1.3}

// open fills the settings in from the view.
func (p *settingsPage) open(v *view) {
	s := v.settings
	p.udp.Value, p.ipv6.Value, p.localDiscovery.Value = s.UDP, s.IPv6, s.LocalDiscovery
	p.background.Value, p.dark.Value = s.Background, s.Dark
	p.proxy.Value = s.Proxy
	if p.proxy.Value == "" {
		p.proxy.Value = "none"
	}
	p.proxyHost.SetText(s.ProxyHost)
	p.proxyPort.SetText("")
	if s.ProxyPort != 0 {
		p.proxyPort.SetText(strconv.Itoa(int(s.ProxyPort)))
	}
	p.nodes.SetText(s.Nodes)
	p.textSize.Value = "normal"
	for k, size := range textSizes {
		if size == s.TextSize {
			p.textSize.Value = k
		}
	}
	p.proxyHost.SingleLine, p.proxyPort.SingleLine = true, true
	p.password.SingleLine, p.password2.SingleLine = true, true
	p.password.Mask, p.password2.Mask = '•', '•'
	p.password.SetText("")
	p.password2.SetText("")
	p.removePassword.asking = false
	p.deleteProfile.asking = false
	p.deleteMsg = ""
	p.networkErr, p.passwordMsg = "", ""
}

// settings are the settings as the user set them.
func (p *settingsPage) settings() (settings, error) {
	s := settings{
		UDP:            p.udp.Value,
		IPv6:           p.ipv6.Value,
		LocalDiscovery: p.localDiscovery.Value,
		Background:     p.background.Value,
		Dark:           p.dark.Value,
		TextSize:       textSizes[p.textSize.Value],
		ProxyHost:      strings.TrimSpace(p.proxyHost.Text()),
		Nodes:          strings.TrimSpace(p.nodes.Text()),
	}
	if p.proxy.Value != "none" {
		s.Proxy = p.proxy.Value
	}
	if port := strings.TrimSpace(p.proxyPort.Text()); port != "" {
		n, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			return s, errors.New("the proxy's port is a number up to 65535")
		}
		s.ProxyPort = uint16(n)
	}
	return s, s.check()
}

func (u *ui) settingsRows(gtx layout.Context, v *view) []layout.Widget {
	p := &u.settings
	apply := false
	for _, b := range []*widget.Bool{&p.udp, &p.ipv6, &p.localDiscovery, &p.background, &p.dark} {
		if b.Update(gtx) {
			apply = true
		}
	}
	if p.textSize.Update(gtx) || p.saveNetwork.Clicked(gtx) {
		apply = true
	}
	if apply {
		p.networkErr = ""
		if s, err := p.settings(); err != nil {
			p.networkErr = err.Error()
		} else if err := u.home.apply(s); err != nil {
			p.networkErr = err.Error()
		}
	}
	if p.setPassword.Clicked(gtx) {
		pass := p.password.Text()
		switch {
		case pass == "":
			p.passwordMsg = "Type the password twice."
		case pass != p.password2.Text():
			p.passwordMsg = "The two passwords aren't the same."
		default:
			if err := u.home.setPassword(pass); err != nil {
				p.passwordMsg = err.Error()
			} else {
				p.passwordMsg = "Your profile has the new password."
				p.password.SetText("")
				p.password2.SetText("")
			}
		}
	}
	if p.removePassword.update(gtx) {
		if err := u.home.setPassword(""); err != nil {
			p.passwordMsg = err.Error()
		} else {
			p.passwordMsg = "Your profile has no password now."
		}
	}
	if p.export.Clicked(gtx) {
		u.clearNotice()
		u.exportProfile()
	}
	if p.deleteProfile.update(gtx) {
		if err := u.home.deleteProfile(u.home.shown); err != nil {
			p.deleteMsg = err.Error()
		} else {
			u.navigate(pageFriends)
		}
	}

	passwordText := "With a password, your profile and chats on this device are encrypted, and the app asks for it when it starts."
	if v.encrypted {
		passwordText = "Your profile has a password: the app asks for it when it starts."
	}
	rows := []layout.Widget{
		gap(32),
		material.H6(u.th, "Password").Layout,
		material.Body2(u.th, passwordText).Layout,
		gap(8),
		u.field(&p.password, "New password"),
		gap(8),
		u.field(&p.password2, "The same again"),
		gap(8),
		material.Button(u.th, &p.setPassword, "Set the password").Layout,
	}
	if v.encrypted {
		rows = append(rows, gap(8), func(gtx layout.Context) layout.Dimensions {
			return u.layoutConfirmation(gtx, &p.removePassword, "Remove the password",
				"Anyone with this device can then read your profile and chats.", "Remove it")
		})
	}
	rows = append(rows,
		gap(8),
		func(gtx layout.Context) layout.Dimensions {
			if p.passwordMsg == "" {
				return layout.Dimensions{}
			}
			return material.Body2(u.th, p.passwordMsg).Layout(gtx)
		},
		gap(32),
		material.H6(u.th, "Profile").Layout,
		material.Body2(u.th, "Export saves your profile to a file, to use it in another Tox app or keep a copy. It has your password, if you set one.").Layout,
		gap(8),
		u.lightButton(&p.export, "Export the profile"),
	)
	// The last profile stays, as does one in a call.
	if u.home.fixed == "" && len(u.home.profiles()) > 1 && u.home.caller() != u.home.shown {
		rows = append(rows, gap(8), func(gtx layout.Context) layout.Dimensions {
			return u.layoutConfirmation(gtx, &p.deleteProfile, "Delete this profile",
				"Its Tox ID and its chats go from this device, and its friends can't reach it again.", "Delete it")
		})
		if p.deleteMsg != "" {
			rows = append(rows, gap(8), material.Body2(u.th, p.deleteMsg).Layout)
		}
	}
	rows = append(rows,
		gap(32),
		material.H6(u.th, "Look").Layout,
		material.CheckBox(u.th, &p.dark, "Dark").Layout,
		material.Body2(u.th, "Text size").Layout,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(material.RadioButton(u.th, &p.textSize, "small", "Small").Layout),
				layout.Rigid(material.RadioButton(u.th, &p.textSize, "normal", "Normal").Layout),
				layout.Rigid(material.RadioButton(u.th, &p.textSize, "large", "Large").Layout),
				layout.Rigid(material.RadioButton(u.th, &p.textSize, "larger", "Larger").Layout),
			)
		},
		gap(32),
		material.H6(u.th, "Network").Layout,
		material.Body2(u.th, "Changing these reconnects.").Layout,
		gap(8),
		material.CheckBox(u.th, &p.udp, "UDP (off: connect through TCP relays only)").Layout,
		material.CheckBox(u.th, &p.ipv6, "IPv6").Layout,
		material.CheckBox(u.th, &p.localDiscovery, "Find Tox users on the local network").Layout,
		gap(16),
		material.Body2(u.th, "Proxy").Layout,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(material.RadioButton(u.th, &p.proxy, "none", "None").Layout),
				layout.Rigid(material.RadioButton(u.th, &p.proxy, "socks5", "SOCKS5").Layout),
				layout.Rigid(material.RadioButton(u.th, &p.proxy, "http", "HTTP").Layout),
			)
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(3, u.field(&p.proxyHost, "Host")),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Flexed(1, u.field(&p.proxyPort, "Port")),
			)
		},
		gap(16),
		material.Body2(u.th, "Your own bootstrap nodes, a line each: host, port and DHT key").Layout,
		u.field(&p.nodes, "tox.example.org 33445 F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67"),
		gap(8),
		material.Button(u.th, &p.saveNetwork, "Save the proxy and nodes").Layout,
	)
	if closesApps {
		rows = append(rows,
			gap(24),
			material.H6(u.th, "Background").Layout,
			material.CheckBox(u.th, &p.background, "Stay connected while the app is closed").Layout,
		)
	}
	rows = append(rows, gap(8), u.errorText(p.networkErr), gap(24),
		func(gtx layout.Context) layout.Dimensions {
			l := material.Caption(u.th, "Go Tox "+version)
			l.Color = secondaryText
			return l.Layout(gtx)
		})
	return rows
}

// A palette is the app's colours, light or dark.
type palette struct {
	bg, fg, divider, outgoingBg, incomingBg, requestBg, secondaryText color.NRGBA
}

var (
	lightPalette = palette{
		bg:            color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		fg:            color.NRGBA{A: 0xff},
		divider:       color.NRGBA{R: 0xe0, G: 0xe0, B: 0xe0, A: 0xff},
		outgoingBg:    color.NRGBA{R: 0xd9, G: 0xe6, B: 0xff, A: 0xff},
		incomingBg:    color.NRGBA{R: 0xef, G: 0xef, B: 0xef, A: 0xff},
		requestBg:     color.NRGBA{R: 0xff, G: 0xf4, B: 0xd6, A: 0xff},
		secondaryText: color.NRGBA{R: 0x75, G: 0x75, B: 0x75, A: 0xff},
	}
	darkPalette = palette{
		bg:            color.NRGBA{R: 0x12, G: 0x12, B: 0x14, A: 0xff},
		fg:            color.NRGBA{R: 0xe6, G: 0xe6, B: 0xe6, A: 0xff},
		divider:       color.NRGBA{R: 0x33, G: 0x33, B: 0x38, A: 0xff},
		outgoingBg:    color.NRGBA{R: 0x23, G: 0x34, B: 0x5c, A: 0xff},
		incomingBg:    color.NRGBA{R: 0x2a, G: 0x2a, B: 0x30, A: 0xff},
		requestBg:     color.NRGBA{R: 0x3d, G: 0x35, B: 0x20, A: 0xff},
		secondaryText: color.NRGBA{R: 0x9e, G: 0x9e, B: 0xa6, A: 0xff},
	}
)

// setLook gives the UI the colours and text size of the settings.
func (u *ui) setLook(s settings) {
	pal := lightPalette
	if s.Dark {
		pal = darkPalette
	}
	divider, outgoingBg, incomingBg, requestBg, secondaryText = pal.divider, pal.outgoingBg, pal.incomingBg, pal.requestBg, pal.secondaryText
	u.th.Palette.Bg, u.th.Palette.Fg = pal.bg, pal.fg
	size := s.TextSize
	if size == 0 {
		size = 1
	}
	u.th.TextSize = unit.Sp(16 * size)
}
