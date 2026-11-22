package main

import (
	"errors"
	"io"
	"path/filepath"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// startPage opens a profile: it picks one, asks for its password, and makes
// and imports them.
type startPage struct {
	list    widget.List
	clicks  map[string]*widget.Clickable
	chosen  string
	pass    widget.Editor
	unlock  widget.Clickable
	newName widget.Editor
	create  widget.Clickable
	choose  widget.Clickable
	// imported is a profile file to import, once the user names it.
	imported   []byte
	importName widget.Editor
	importPass widget.Editor
	doImport   widget.Clickable
	err        string
}

// An importedProfile is a profile file the user chose, for the UI goroutine.
type importedProfile struct {
	name string
	data []byte
}

func (u *ui) layoutStart(gtx layout.Context) layout.Dimensions {
	p := &u.start
	p.list.Axis = layout.Vertical
	if p.clicks == nil {
		p.clicks = map[string]*widget.Clickable{}
	}
	for _, e := range []*widget.Editor{&p.pass, &p.newName, &p.importName, &p.importPass} {
		e.SingleLine = true
	}
	p.pass.Mask, p.importPass.Mask = '•', '•'
	p.pass.Submit = true

	profiles := u.home.profiles()
	encrypted := func(name string) bool {
		for _, pi := range profiles {
			if pi.name == name {
				return pi.encrypted
			}
		}
		return false
	}
	// Inside the app, a profile opened here is shown in it.
	inside := u.core != nil
	opened := func() {
		if inside {
			u.navigate(pageFriends)
		}
	}
	open := func(name, password string) {
		if err := u.home.open(name, password); err != nil {
			p.err = err.Error()
			return
		}
		p.err, p.chosen = "", ""
		p.pass.SetText("")
		opened()
	}
	for _, pi := range profiles {
		if state(p.clicks, pi.name).Clicked(gtx) {
			p.err = ""
			if pi.encrypted {
				p.chosen = pi.name
				p.pass.SetText("")
				gtx.Execute(focusCmd(&p.pass))
			} else {
				open(pi.name, "")
			}
		}
	}
	unlock := p.unlock.Clicked(gtx)
	for {
		ev, ok := p.pass.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			unlock = true
		}
	}
	if unlock && p.chosen != "" {
		open(p.chosen, p.pass.Text())
	}
	if p.create.Clicked(gtx) {
		if err := u.home.create(p.newName.Text()); err != nil {
			p.err = err.Error()
		} else {
			p.newName.SetText("")
			opened()
		}
	}
	if p.choose.Clicked(gtx) {
		u.chooseProfile()
	}
	if imp := u.takeImported(); imp != nil {
		p.imported = imp.data
		p.importName.SetText(imp.name)
		p.importPass.SetText("")
		p.err = ""
	}
	if p.doImport.Clicked(gtx) && p.imported != nil {
		if err := u.home.importProfile(p.importName.Text(), p.imported, p.importPass.Text()); err != nil {
			p.err = err.Error()
		} else {
			p.imported = nil
			opened()
		}
	}

	var rows []layout.Widget
	if p.chosen != "" && encrypted(p.chosen) {
		rows = append(rows,
			material.H6(u.th, p.chosen).Layout,
			material.Body2(u.th, "This profile has a password.").Layout,
			u.field(&p.pass, "Password"),
			gap(8),
			material.Button(u.th, &p.unlock, "Open").Layout,
			gap(8),
			u.errorText(p.err),
			gap(24),
		)
	}
	if u.home.fixed == "" {
		rows = append(rows, material.H6(u.th, "Profiles").Layout)
		for _, pi := range profiles {
			label := pi.name
			if pi.encrypted {
				label += " (password)"
			}
			rows = append(rows, func(gtx layout.Context) layout.Dimensions {
				return state(p.clicks, pi.name).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 8, Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return u.layoutAvatar(gtx, nil, pi.name, pi.name, 40)
							}),
							layout.Rigid(layout.Spacer{Width: 12}.Layout),
							layout.Flexed(1, material.Body1(u.th, label).Layout),
						)
					})
				})
			})
		}
		rows = append(rows,
			gap(24),
			material.H6(u.th, "New profile").Layout,
			u.field(&p.newName, "Its name, for you"),
			gap(8),
			material.Button(u.th, &p.create, "Make it").Layout,
			gap(24),
			material.H6(u.th, "Import a profile").Layout,
			material.Body2(u.th, "A profile.tox from Go Tox or another Tox app.").Layout,
			gap(8),
		)
		if p.imported == nil {
			rows = append(rows, u.lightButton(&p.choose, "Choose the file"))
		} else {
			rows = append(rows,
				u.field(&p.importName, "Its name, for you"),
				gap(8),
				u.field(&p.importPass, "Its password, if it has one"),
				gap(8),
				material.Button(u.th, &p.doImport, "Import").Layout,
			)
		}
		if p.chosen == "" || !encrypted(p.chosen) {
			rows = append(rows, gap(16), u.errorText(p.err))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if inside {
				return u.header(gtx, true, "Profiles", "")
			}
			return u.header(gtx, false, "Go Tox", "")
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return u.layoutForm(gtx, &p.list, rows)
		}),
	)
}

// chooseProfile asks the user for a profile file to import.
func (u *ui) chooseProfile() {
	if u.explorer == nil {
		return
	}
	go func() {
		r, err := u.explorer.ChooseFile(".tox")
		if err != nil {
			u.report(err)
			return
		}
		defer r.Close()
		data, err := io.ReadAll(r)
		if err != nil {
			u.report(err)
			return
		}
		if len(data) == 0 {
			u.report(errors.New("that file is empty"))
			return
		}
		name := strings.TrimSuffix(filepath.Base(fileName(r)), ".tox")
		u.files.mu.Lock()
		u.files.imported = &importedProfile{name, data}
		u.files.mu.Unlock()
		u.invalidate()
	}()
}

// takeImported returns the profile file the user chose, once.
func (u *ui) takeImported() *importedProfile {
	u.files.mu.Lock()
	defer u.files.mu.Unlock()
	imp := u.files.imported
	u.files.imported = nil
	return imp
}

// exportProfile asks the user where to save the profile, and saves it there.
func (u *ui) exportProfile() {
	if u.explorer == nil || u.core == nil {
		return
	}
	data, err := u.core.profileData()
	if err != nil {
		u.report(err)
		return
	}
	name := u.home.shown + ".tox"
	go func() {
		w, err := u.explorer.CreateFile(name)
		if err != nil {
			u.report(err)
			return
		}
		_, err = w.Write(data)
		if cerr := w.Close(); err == nil {
			err = cerr
		}
		u.report(err)
	}()
}
