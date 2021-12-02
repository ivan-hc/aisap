package aisap

import (
	"bytes"
	"io/ioutil"
	"strings"
	"strconv"
	"errors"

	helpers  "github.com/mgord9518/aisap/helpers"
	permissions "github.com/mgord9518/aisap/permissions"
	profiles "github.com/mgord9518/aisap/profiles"
	ini	     "gopkg.in/ini.v1"
)

// GetPerms attemps to read permissions from a provided desktop entry, if
// fail, it will return an error and empty *permissions.AppImagePerms
func getPermsFromEntry(entryFile string) (*permissions.AppImagePerms, error) {
	aiPerms := permissions.AppImagePerms{}

	if !helpers.FileExists(entryFile) {
		return nil, errors.New("failed to find requested desktop entry! ("+entryFile+")")
	}

	e, err := ioutil.ReadFile(entryFile)
	if err != nil { return nil, err }

	// Replace ';' with fullwidth semicolon so that the ini package doesn't consider it a break
	e = bytes.ReplaceAll(e, []byte(";"), []byte("；"))

	entry, err := ini.Load(e)
	if err != nil { return nil, err }

	aiPerms, err = loadPerms(aiPerms, entry)

	if err != nil {
		aiPerms.Level = -1
	}

	return &aiPerms, err
}

// Attempt to fetch permissions from the AppImage itself, fall back on aisap
// internal permissinos library
func getPermsFromAppImage(ai *AppImage) (*permissions.AppImagePerms, error) {
	var err error
	var present bool

	aiPerms := permissions.AppImagePerms{}

	// Use the aisap internal profile if it exists
	// If not, set its level as invalid
	if aiPerms, present = profiles.Profiles[strings.ToLower(ai.Name)]; present {
		return &aiPerms, nil
	} else {
		aiPerms.Level = -1
	}

	aiPerms, err = loadPerms(aiPerms, ai.Desktop)
	if err != nil {return &aiPerms, err}

	return &aiPerms, err
}

func loadPerms(p permissions.AppImagePerms, f *ini.File) (permissions.AppImagePerms, error) {
	err = nil

	// Get permissions from entry keys
	level       := f.Section("Required Permissions").Key("Level").Value()
	filePerms   := f.Section("Required Permissions").Key("Files").Value()
	devicePerms := f.Section("Required Permissions").Key("Devices").Value()
	socketPerms := f.Section("Required Permissions").Key("Sockets").Value()
	sharePerms  := f.Section("Required Permissions").Key("Share").Value()

	// If the AppImage desktop entry has permission flags, overwrite the
	// profile flags
	if level != "" {
		l, err := strconv.Atoi(level)

		if err != nil || l < 0 || l > 3 {
			p.Level = -1
			err = errors.New("invalid permissions level (must be 0-3)")
		} else {
			p.Level = l
		}
	} else {
		p.Level = -1
		err = errors.New("profile does not have required flag `Level` under section [Required Permissions]")
	}
	if len(filePerms) > 0 {
		p.Files = helpers.DesktopSlice(filePerms)
	}
	if len(devicePerms) > 0 {
		p.Devices = helpers.DesktopSlice(devicePerms)
	}
	if len(socketPerms) > 0 {
		p.Sockets = helpers.DesktopSlice(socketPerms)
	}
	if len(sharePerms) > 0 {
		p.Share = helpers.DesktopSlice(sharePerms)
	}

	// If all keys are still empty, throw an error
	if len(p.Files)    == 0 && len(p.Devices) == 0 &&
		len(p.Sockets) == 0 && len(p.Share)   == 0 {
		err = errors.New("entry contains no permissions")
	}

	// Add `:ro` if file doesn't specify
	for i := range(p.Files) {
		ex := p.Files[i][len(p.Files[i])-3:]

		if len(strings.Split(p.Files[i], ":")) < 2 ||
		ex != ":ro" && ex != ":rw" {
			p.Files[i] = p.Files[i]+":ro"
		}
	}

	return p, err
}
