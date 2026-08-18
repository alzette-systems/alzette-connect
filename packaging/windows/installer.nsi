Unicode true
RequestExecutionLevel user
SetCompressor /SOLID lzma

!include "MUI2.nsh"

!ifndef APP_VERSION
  !define APP_VERSION "0.1.0-demo"
!endif
!ifndef APP_BINARY
  !error "APP_BINARY is required"
!endif
!ifndef APP_ICON
  !error "APP_ICON is required"
!endif
!ifndef NOTICE_FILE
  !error "NOTICE_FILE is required"
!endif
!ifndef OUTPUT_FILE
  !error "OUTPUT_FILE is required"
!endif

Name "Alzette Connect ${APP_VERSION}"
OutFile "${OUTPUT_FILE}"
InstallDir "$LOCALAPPDATA\Programs\Alzette Connect"
InstallDirRegKey HKCU "Software\AlzetteSystems\AlzetteConnect" "InstallLocation"
Icon "${APP_ICON}"
UninstallIcon "${APP_ICON}"
ShowInstDetails show
ShowUninstDetails show

!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\alzette-connect.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Open Alzette Connect"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Section "Alzette Connect" SEC_APP
  SetOutPath "$INSTDIR"
  File /oname=alzette-connect.exe "${APP_BINARY}"
  File /oname=UNSIGNED-DEMO.txt "${NOTICE_FILE}"
  File /oname=alzette-connect.ico "${APP_ICON}"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  WriteRegStr HKCU "Software\AlzetteSystems\AlzetteConnect" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AlzetteConnect" "DisplayName" "Alzette Connect"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AlzetteConnect" "DisplayVersion" "${APP_VERSION}"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AlzetteConnect" "Publisher" "Alzette Systems"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AlzetteConnect" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AlzetteConnect" "DisplayIcon" "$INSTDIR\alzette-connect.exe"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AlzetteConnect" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AlzetteConnect" "NoModify" 1
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AlzetteConnect" "NoRepair" 1
  CreateDirectory "$SMPROGRAMS\Alzette Connect"
  CreateShortcut "$SMPROGRAMS\Alzette Connect\Alzette Connect.lnk" "$INSTDIR\alzette-connect.exe" "" "$INSTDIR\alzette-connect.ico"
SectionEnd

Section "Uninstall"
  Delete "$SMPROGRAMS\Alzette Connect\Alzette Connect.lnk"
  RMDir "$SMPROGRAMS\Alzette Connect"
  Delete "$INSTDIR\alzette-connect.exe"
  Delete "$INSTDIR\UNSIGNED-DEMO.txt"
  Delete "$INSTDIR\alzette-connect.ico"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AlzetteConnect"
  DeleteRegKey HKCU "Software\AlzetteSystems\AlzetteConnect"
SectionEnd
