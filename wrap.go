package aisap

import (
	"errors"
	"os/exec"
	"strings"
	"os"
	"strconv"

	helpers  "github.com/mgord9518/aisap/helpers"
	profiles "github.com/mgord9518/aisap/profiles"
	xdg	     "github.com/adrg/xdg"
)

// Run the AppImage with zero sandboxing
func Run(ai *AppImage, args []string) error {
	err = setupRun(ai)
	if err != nil { return err }

	cmd := exec.Command(ai.MountDir()+"/AppRun", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin  = os.Stdin
	cmd.Start()
	err = cmd.Wait()
	if err != nil { return err }

	// Clean up after the app is closed
	// Sleep is needed to wait until the AppImage is unmounted before deleting the temporary dir
	err = UnmountAppImage(ai)
	if err != nil {return err}
	err = os.RemoveAll(ai.TempDir())
	return err
}

// Wrap is a re-implementation of the aibwrap shell script, allowing execution of AppImages through bwrap
func Sandbox(ai *AppImage, args []string) error {
	bwrapArgs := GetWrapArgs(ai.Perms)

	if _, err := exec.LookPath("bwrap"); err != nil {
		return errors.New("bubblewrap not found! It's required to use sandboing")
	}

	err = setupRun(ai)
	if err != nil { return err }

	// Bind the fake /home and /tmp dirs
	bwrapArgs = append([]string{
		"--bind", dataDir, "/home/"+usern,
		"--bind", ai.TempDir(), "/tmp",
	}, bwrapArgs...)

	bwrapArgs = append(bwrapArgs, "--",
		"/tmp/.mount_"+ai.RunId()+"/AppRun",
	)

	bwrapArgs = append(bwrapArgs, args...)

	bwrap := exec.Command("bwrap", bwrapArgs...)

	bwrap.Stdout = os.Stdout
	bwrap.Stderr = os.Stderr
	bwrap.Stdin  = os.Stdin
	bwrap.Start()
	err = bwrap.Wait()
	if err != nil { return err }

	// Clean up after the app is closed
	err = UnmountAppImage(ai)
	if err != nil {return err}
	err = os.RemoveAll(ai.TempDir())
	return err
}

func setupRun(ai *AppImage) error {
	if dataDir == "" {
		dataDir = ai.Path+".home"
	}

	if !helpers.DirExists(dataDir) {
		err := os.MkdirAll(dataDir, 0744)
		if err != nil { return err }
	}

	if !helpers.DirExists(dataDir+"/.local/share/appimagekit/") {
		err := os.MkdirAll(dataDir+"/.local/share/appimagekit/", 0744)
		if err != nil { return err }
	}

	// Tell AppImages not to ask for integration
	noIntegrate, err := os.Create(dataDir+"/.local/share/appimagekit/no_desktopintegration")
	noIntegrate.Close()

	// Set required vars to correctly mount our target AppImage
	// If sandboxed, these values will be overwritten
	os.Setenv("TMPDIR", ai.TempDir())
	os.Setenv("HOME",   dataDir)

	return err
}

func GetWrapArgs(perms *profiles.AppImagePerms) []string {
	loadName(perms.Level)
	// Basic arguments to be used at all sandboxing levels
	cmdArgs := []string{
			"--setenv",	  "TMPDIR",              "/tmp",
			"--setenv",	  "HOME",                homed,
			"--setenv",	  "XDG_DESKTOP_DIR",     homed+"/Desktop",
			"--setenv",	  "XDG_DOWNLOAD_DIR",    homed+"/Downloads",
			"--setenv",	  "XDG_DOCUMENTS_DIR",   homed+"/Documents",
			"--setenv",	  "XDG_MUSIC_DIR",       homed+"/Music",
			"--setenv",	  "XDG_PICTURES_DIR",    homed+"/Pictures",
			"--setenv",	  "XDG_VIDEOS_DIR",      homed+"/Videos",
			"--setenv",	  "XDG_TEMPLATES_DIR",   homed+"/Templates",
			"--setenv",	  "XDG_PUBLICSHARE_DIR", homed+"/Templates",
			"--setenv",	  "XDG_DATA_HOME",       homed+"/.local/share",
			"--setenv",	  "XDG_CONFIG_HOME",     homed+"/.config",
			"--setenv",	  "XDG_CACHE_HOME",      homed+"/.cache",
			"--setenv",	  "LOGNAME",             usern,
			"--setenv",	  "USER",                usern,
			"--uid",       uid,
			"--unshare-pid",
			"--unshare-user-try",
			"--die-with-parent",
			"--new-session",
			"--dev",		 "/dev",
			"--proc",        "/proc",
			"--ro-bind",	 "/opt",              "/opt",
			"--ro-bind",	 "/bin",              "/bin",
			"--ro-bind",	 "/lib",              "/lib",
			"--ro-bind-try", "/lib32",            "/lib32",
			"--ro-bind-try", "/lib64",            "/lib64",
			"--ro-bind",	 "/usr/bin",          "/usr/bin",
			"--ro-bind",	 "/usr/lib",          "/usr/lib",
			"--ro-bind-try", "/usr/lib32",        "/usr/lib32",
			"--ro-bind-try", "/usr/lib64",        "/usr/lib64",
			"--ro-bind-try", "/usr/share/fonts",  "/usr/share/fonts",
			"--ro-bind-try", "/usr/share/icons",  "/usr/share/icons",
			"--ro-bind-try", "/usr/share/themes", "/usr/share/themes",
	}

	ruid := strconv.Itoa(os.Getuid()) // Real UID, for level 1 RUID and UID are the same value

	// Convert device perms to bwrap format
	for _, v := range(perms.Devices) {
		cmdArgs = append(cmdArgs, "--dev-bind-try", "/dev/"+v, "/dev/"+v)
	}

	// Convert XDG standards into real paths eg: `xdg-desktop` becomes `~/Desktop`
	stdDirs := getXdg(perms.Files, perms.Level)
	// Convert directory permissions to bwrap flags
	for i, _ := range stdDirs {
		dir := strings.Split(i, ":")[0]

		// Convert "rw"/"ro" into bwrap command line syntax so we can call it
		if strings.Split(i, ":")[1] == "rw" {
			cmdArgs = append(cmdArgs, "--bind-try", dir, stdDirs[i])
		} else {
			cmdArgs = append(cmdArgs, "--ro-bind-try", dir, stdDirs[i])
		}
	}

	// Level 1 is minimal sandboxing, grants access to most system files, all devices and only really attempts to isolate home files
	if perms.Level == 1 {
		cmdArgs = append(cmdArgs, []string{
			"--dev-bind",    "/dev", "/dev",
			"--ro-bind",	 "/sys", "/sys",
			"--ro-bind",	 "/usr", "/usr",
			"--ro-bind-try", "/etc", "/etc",
			"--ro-bind-try", xdg.Home+"/.fonts",                     homed+"/.fonts",
			"--ro-bind-try", xdg.ConfigHome+"/fontconfig",           homed+"/.config/fontconfig",
			"--ro-bind-try", xdg.ConfigHome+"/gtk-3.0/gtk.css",      homed+"/.config/gtk-3.0/gtk.css",
			"--ro-bind-try", xdg.ConfigHome+"/gtk-3.0/settings.ini", homed+"/.config/gtk-3.0/settings.ini",
		}...)
	// Level 2 grants access to fewer system files, and all themes
	// Likely to add more files here for compatability.
	// This should be the typical level for created profiles
	} else if perms.Level == 2 {
		cmdArgs = append(cmdArgs, []string{
			"--ro-bind-try", "/etc/fonts",              "/etc/fonts",
			"--ro-bind-try", "/etc/ssl",                "/etc/ssl",
			"--ro-bind-try", "/usr/share/fontconfig",   "/usr/share/fontconfig",
			"--ro-bind-try", "/usr/share/applications", "/usr/share/applications",
			"--ro-bind-try", "/usr/share/mime",         "/usr/share/mime",
			"--ro-bind-try", "/usr/share/libdrm",       "/usr/share/librdm",
			"--ro-bind-try", "/usr/share/glvnd",        "/usr/share/glvnd",
			"--ro-bind-try", "/usr/share/glib-2.0",     "/usr/share/glib-2.0",
			"--ro-bind-try", xdg.Home+"/.fonts",           homed+"/.fonts",
			"--ro-bind-try", xdg.ConfigHome+"/fontconfig", homed+"/.config/fontconfig",
			"--ro-bind-try", xdg.ConfigHome+"/gtk-3.0",    homed+"/.config/gtk-3.0",
		}...)
	}

	// These vars will only be used if x11 socket is granted access
	xAuthority := os.Getenv("XAUTHORITY")
	xDisplay := strings.ReplaceAll(os.Getenv("DISPLAY"), ":", "")

	// Used if this socket is enabled
	var sockets = map[string][]string {
		// For some reason sometimes it doesn't work when binding X0 to another socket
		// ...but sometimes it does
		"x11": {
			"--ro-bind",	 xAuthority,                      homed+"/.Xauthority",
			"--ro-bind",	 tempDir+"/.X11-unix/X"+xDisplay, "/tmp/.X11-unix/X"+xDisplay,
			"--ro-bind-try", "/usr/share/X11",                "/usr/share/X11",
			"--setenv",      "XAUTHORITY",                    homed+"/.Xauthority",
			"--setenv",      "DISPLAY",                       ":"+xDisplay,
		},
		"pulseaudio": {
			"--ro-bind-try", "/run/user/"+ruid+"/pulse", "/run/user/"+ruid+"/pulse",
		},
	}

	for socket, _ := range(sockets) {
		_, present := helpers.Contains(perms.Sockets, socket)
		if present {
			cmdArgs = append(cmdArgs, sockets[socket]...)
		}
	}

	var unshares = map[string]string {
		"user":    "--unshare-user-try",
		"ipc":     "--unshare-ipc",
		"pid":     "--unshare-pid",
		"network": "--unshare-net",
		"uts":     "--unshare-uts",
		"cgroup":  "--unshare-cgroup-try",
	}

	for s, _ := range unshares {
		_, present := helpers.Contains(perms.Share, s)
		if present {
			// Single exception, network share requires `/etc/resolv.conf`
			if s == "network" {
				cmdArgs = append(cmdArgs, "--share-net", "--ro-bind", "/etc/resolv.conf", "/etc/resolv.conf")
			}
		} else {
			cmdArgs = append(cmdArgs, unshares[s])
		}
	}

	return cmdArgs
}

// Parses requested file and directories into bwrap flags
func getXdg(s []string, level int) map[string]string {
    var genericDir string
    dirPerms := make(map[string]string)

    // Map out the XDG directories
    var xdgDirs = map[string]string {
        "xdg-home":        xdg.Home,
        "xdg-desktop":     xdg.UserDirs.Desktop,
        "xdg-download":    xdg.UserDirs.Download,
        "xdg-documents":   xdg.UserDirs.Documents,
        "xdg-music":       xdg.UserDirs.Music,
        "xdg-pictures":    xdg.UserDirs.Pictures,
        "xdg-videos":      xdg.UserDirs.Videos,
        "xdg-templates":   xdg.UserDirs.Templates,
        "xdg-publicshare": xdg.UserDirs.PublicShare,
        "xdg-config":      xdg.ConfigHome,
        "xdg-cache":       xdg.CacheHome,
        "xdg-data":        xdg.DataHome,
    }

    // Anonymize directories by giving them generic names in case the user has
    // changed the location of their XDG-dirs (unlikely but happens)
    var xdgGeneric = map[string]string {
        "xdg-home":        homed,
        "xdg-desktop":     homed+"/Desktop",
        "xdg-download":    homed+"/Downloads",
        "xdg-documents":   homed+"/Documents",
        "xdg-music":       homed+"/Music",
        "xdg-pictures":    homed+"/Pictures",
        "xdg-videos":      homed+"/Videos",
        "xdg-templates":   homed+"/Templaates",
        "xdg-publicshare": homed+"/Share",
        "xdg-config":      homed+"/.config",
        "xdg-cache":       homed+"/.cache",
        "xdg-data":        homed+"/.local/share",
    }

    for i, _ := range s {
        str := s[i]

        // Replace the xdg-* strings with the corresponding directories on the user's machine
        for key, val := range xdgDirs {

            // If length of key bigger than requested directory or not equal to it continue because there is no reason to look at it further
            if len(key) > len(str) || key != str[:len(key)] {
				// Replace real home directory with sandboxed one
                genericDir = strings.Split(str, ":")[0]
				genericDir = strings.Replace(genericDir, xdg.Home, homed, 1)
                continue
            }

            // If the last byte of the requested path shortened to key length is a '/' or ':' we know it's the parent dir, so resolve it using the xdgDirs map
            c := str[len(key)]  // The final byte of the key (used for splitting)
            r := str[len(key):] // Every string after that byte
            if c == byte('/') || c == byte(':') {
                genericDir = xdgGeneric[key] + strings.Split(r, ":")[0]
                s[i] = strings.Replace(str, key, val, 1)
                break
            } else {
                genericDir = strings.Split(str, ":")[0]
            }
        }

        dirPerms[s[i]] = genericDir
    }

    return dirPerms
}
