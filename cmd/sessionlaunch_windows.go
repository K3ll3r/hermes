package cmd

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/google/deck"
	"golang.org/x/sys/windows"
)

var (
	modwtsapi32 = windows.NewLazySystemDLL("wtsapi32.dll")
	moduserenv  = windows.NewLazySystemDLL("userenv.dll")

	procWTSEnumerateSessions = modwtsapi32.NewProc("WTSEnumerateSessionsW")
	procWTSFreeMemory        = modwtsapi32.NewProc("WTSFreeMemory")
	procWTSQueryUserToken    = modwtsapi32.NewProc("WTSQueryUserToken")
	procCreateEnvBlock       = moduserenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvBlock      = moduserenv.NewProc("DestroyEnvironmentBlock")
)

const (
	wtsActive        = 0
	tokenPrimary     = 1
	securityImperson = 2
	createUnicodeEnv = 0x00000400
	createNoWindow   = 0x08000000
)

type wtsSessionInfo struct {
	SessionID      uint32
	WinStationName *uint16
	State          uint32
}

// isPrivileged reports whether the current process is running as SYSTEM.
// MSI custom actions run as SYSTEM; manual elevated Admin installs do not
// trigger session launch (by design: the user's own session already has
// the installer running interactively).
func isPrivileged() bool {
	t, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer t.Close()

	user, err := t.GetTokenUser()
	if err != nil {
		return false
	}

	return user.User.Sid.IsWellKnown(windows.WinLocalSystemSid)
}

func launchInUserSessions(exe string, args []string) {
	sessions, err := enumerateActiveSessions()
	if err != nil {
		deck.Warningf("session launch: enumerate sessions: %v", err)
		return
	}

	cmdLine, err := buildCommandLine(exe, args)
	if err != nil {
		deck.Warningf("session launch: build command line: %v", err)
		return
	}

	launched := make(map[string]bool)
	for _, sid := range sessions {
		username, err := launchInSession(sid, exe, cmdLine)
		if err != nil {
			deck.Warningf("session launch: session %d: %v", sid, err)
			continue
		}
		if launched[username] {
			deck.Infof("session launch: session %d: skipped (already launched for %s)", sid, username)
			continue
		}
		launched[username] = true
		deck.Infof("session launch: started hermes serve in session %d (%s)", sid, username)
	}
}

func enumerateActiveSessions() ([]uint32, error) {
	var (
		pSessionInfo uintptr
		count        uint32
	)

	r1, _, err := procWTSEnumerateSessions.Call(
		0, // WTS_CURRENT_SERVER_HANDLE
		0,
		1,
		uintptr(unsafe.Pointer(&pSessionInfo)),
		uintptr(unsafe.Pointer(&count)),
	)
	if r1 == 0 {
		return nil, fmt.Errorf("WTSEnumerateSessionsW: %w", err)
	}
	defer procWTSFreeMemory.Call(pSessionInfo)

	entrySize := unsafe.Sizeof(wtsSessionInfo{})
	var active []uint32
	for i := uint32(0); i < count; i++ {
		entry := (*wtsSessionInfo)(unsafe.Pointer(pSessionInfo + uintptr(i)*entrySize))
		if entry.State == wtsActive && entry.SessionID != 0 {
			active = append(active, entry.SessionID)
		}
	}
	return active, nil
}

func launchInSession(sessionID uint32, exe, cmdLine string) (string, error) {
	var impersonationToken windows.Handle
	r1, _, err := procWTSQueryUserToken.Call(
		uintptr(sessionID),
		uintptr(unsafe.Pointer(&impersonationToken)),
	)
	if r1 == 0 {
		return "", fmt.Errorf("WTSQueryUserToken: %w", err)
	}
	defer windows.CloseHandle(impersonationToken)

	var userToken windows.Token
	err = windows.DuplicateTokenEx(
		windows.Token(impersonationToken),
		windows.MAXIMUM_ALLOWED,
		nil,
		securityImperson,
		tokenPrimary,
		&userToken,
	)
	if err != nil {
		return "", fmt.Errorf("DuplicateTokenEx: %w", err)
	}
	defer userToken.Close()

	tokenUser, err := userToken.GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("GetTokenUser: %w", err)
	}
	username, _, _, _ := tokenUser.User.Sid.LookupAccount("")

	var envBlock uintptr
	r1, _, err = procCreateEnvBlock.Call(
		uintptr(unsafe.Pointer(&envBlock)),
		uintptr(userToken),
		0,
	)
	if r1 == 0 {
		return "", fmt.Errorf("CreateEnvironmentBlock: %w", err)
	}
	defer procDestroyEnvBlock.Call(envBlock)

	startupInfo := new(windows.StartupInfo)
	startupInfo.Cb = uint32(unsafe.Sizeof(*startupInfo))
	startupInfo.Desktop = windows.StringToUTF16Ptr(`winsta0\default`)
	startupInfo.ShowWindow = windows.SW_HIDE
	startupInfo.Flags = windows.STARTF_USESHOWWINDOW

	var procInfo windows.ProcessInformation

	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return "", fmt.Errorf("UTF16 command line: %w", err)
	}
	exePtr, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return "", fmt.Errorf("UTF16 exe path: %w", err)
	}

	err = windows.CreateProcessAsUser(
		userToken,
		exePtr,
		cmdLinePtr,
		nil,
		nil,
		false,
		createUnicodeEnv|createNoWindow,
		(*uint16)(unsafe.Pointer(envBlock)),
		nil,
		startupInfo,
		&procInfo,
	)
	if err != nil {
		return "", fmt.Errorf("CreateProcessAsUser: %w", err)
	}

	windows.CloseHandle(windows.Handle(procInfo.Thread))
	windows.CloseHandle(windows.Handle(procInfo.Process))
	return username, nil
}

// buildCommandLine constructs a Windows command line string with proper quoting.
// Embedded double quotes are escaped per CreateProcess rules.
func buildCommandLine(exe string, args []string) (string, error) {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, `"`+exe+`"`)
	for _, a := range args {
		escaped := strings.ReplaceAll(a, `"`, `\"`)
		if strings.ContainsAny(a, ` "\`) {
			parts = append(parts, `"`+escaped+`"`)
		} else {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, " "), nil
}
