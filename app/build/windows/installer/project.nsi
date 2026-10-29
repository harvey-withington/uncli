Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## Include the wails tools
####
# UNCLI installs for the user who runs it, in their own folder, so installing
# and updating never need an administrator (decision 0024).
!define REQUEST_EXECUTION_LEVEL "user"
!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.
# The installer quits a running UNCLI, so it offers to start it again.
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}"
!define MUI_FINISHPAGE_RUN_TEXT "Start UNCLI"

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}" # The user's own programs folder: no administrator needed.
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

# UNCLI keeps running in the tray when its window closes, so a running copy
# is asked to quit (as its tray menu's Quit does) before its files are
# replaced or removed. EXE is the copy that asks: the installer brings its
# own, since a copy installed before --quit (0.3.0) opens instead of
# quitting. Any running copy is asked, wherever it's installed (one in
# Program Files, from before per-user installs, too); with none running,
# --quit does nothing. A copy here that doesn't quit (an older one only
# brings its window up) is waited for, then the user is asked to close it.
# UNCLI_INSTALL_TEST set skips all of it, to try an installer into another
# folder while UNCLI runs.
!macro uncli.quitRunning EXE
    ReadEnvStr $R7 UNCLI_INSTALL_TEST
    ${If} $R7 == ""
        ExecWait '"${EXE}" --quit'
    ${EndIf}
    ${If} ${FileExists} "$INSTDIR\${PRODUCT_EXECUTABLE}"
    ${AndIf} $R7 == ""
        StrCpy $R9 0
        ${Do}
            # A running exe can't be opened for writing.
            ClearErrors
            FileOpen $R8 "$INSTDIR\${PRODUCT_EXECUTABLE}" a
            ${IfNot} ${Errors}
                FileClose $R8
                ${Break}
            ${EndIf}
            IntOp $R9 $R9 + 1
            ${If} $R9 > 20
                MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "UNCLI is still running. Quit it (Quit UNCLI on its tray menu, or close its window), then click Retry." /SD IDCANCEL IDRETRY +2
                Abort
                StrCpy $R9 0
            ${Else}
                Sleep 500
            ${EndIf}
        ${Loop}
    ${EndIf}
!macroend

# The uninstall entry, in the user's own registry (SHCTX is HKCU for a
# per-user install), as wails.writeUninstaller writes it machine-wide.
!macro uncli.writeUninstaller
    WriteUninstaller "$INSTDIR\uninstall.exe"

    SetRegView 64
    WriteRegStr SHCTX "${UNINST_KEY}" "Publisher" "${INFO_COMPANYNAME}"
    WriteRegStr SHCTX "${UNINST_KEY}" "DisplayName" "${INFO_PRODUCTNAME}"
    WriteRegStr SHCTX "${UNINST_KEY}" "DisplayVersion" "${INFO_PRODUCTVERSION}"
    WriteRegStr SHCTX "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    WriteRegStr SHCTX "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
    WriteRegStr SHCTX "${UNINST_KEY}" "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
    WriteRegStr SHCTX "${UNINST_KEY}" "QuietUninstallString" "$\"$INSTDIR\uninstall.exe$\" /S"
    WriteRegDWORD SHCTX "${UNINST_KEY}" "NoModify" 1
    WriteRegDWORD SHCTX "${UNINST_KEY}" "NoRepair" 1

    ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
    IntFmt $0 "0x%08X" $0
    WriteRegDWORD SHCTX "${UNINST_KEY}" "EstimatedSize" "$0"
!macroend

!macro uncli.deleteUninstaller
    Delete "$INSTDIR\uninstall.exe"

    SetRegView 64
    DeleteRegKey SHCTX "${UNINST_KEY}"
!macroend

# An older UNCLI installed for everyone (in Program Files, before per-user
# installs) stays where it is until its own uninstaller removes it, which
# needs an administrator. Run by hand, the installer offers to: Windows asks
# for permission. Silent installs (updates) leave it alone. Its uninstaller
# works in the all-users folders, so the window state this copy keeps in
# the user's %AppData% is untouched.
Function uncli.removeMachineCopy
    IfSilent done
    SetRegView 64
    ReadRegStr $R0 HKLM "${UNINST_KEY}" "UninstallString"
    StrCmp $R0 "" done
    MessageBox MB_YESNO|MB_ICONQUESTION "UNCLI is now installed just for you, in $INSTDIR, so installing updates never needs an administrator.$\r$\n$\r$\nAn older copy, installed for everyone in Program Files, is still there. Remove it now? Windows will ask for permission." IDNO done
    StrCpy $R1 $R0 "" 1 # the path, without its quotes
    StrCpy $R1 $R1 -1
    ClearErrors
    ExecShellWait "runas" "$R1" "/S"
    IfErrors done # permission refused
    # It hands over to a copy of itself and returns at once: wait for its
    # entry to go, which it removes last (two minutes at most).
    StrCpy $R2 0
    ${Do}
        ReadRegStr $R0 HKLM "${UNINST_KEY}" "UninstallString"
        ${If} $R0 == ""
        ${OrIf} $R2 > 240
            ${Break}
        ${EndIf}
        IntOp $R2 $R2 + 1
        Sleep 500
    ${Loop}
    done:
FunctionEnd

Section
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    InitPluginsDir
    SetOutPath $PLUGINSDIR
    !insertmacro wails.files
    !insertmacro uncli.quitRunning "$PLUGINSDIR\${PRODUCT_EXECUTABLE}"

    SetOutPath $INSTDIR

    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    # The exe changes in place, so Explorer would keep showing its cached
    # icon on the shortcuts after an update that changes it: tell it that
    # icons changed (SHCNE_ASSOCCHANGED), as restarting Explorer would.
    System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro uncli.writeUninstaller

    Call uncli.removeMachineCopy
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    # The installed copy came with this uninstaller, so it knows --quit.
    !insertmacro uncli.quitRunning "$INSTDIR\${PRODUCT_EXECUTABLE}"
    # Start with Windows (Settings, General) leaves an entry in the user's Run key.
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "UNCLI"

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro uncli.deleteUninstaller
SectionEnd
