; Nudge: per-user Windows setup (Inno Setup 6).
;
; Build it with scripts/build-windows-setup.sh, or by hand:
;   ISCC.exe /DAppVersion=1.0.0 /DSrcDir=C:\path\to\staging /Odist packaging\windows\nudge.iss
;
; Defines:
;   AppVersion     version shown to the user (required), e.g. 1.0.0
;   AppVersionNum  numeric x.y.z.w for the version resource (optional,
;                  defaults to AppVersion; the build script passes it)
;   SrcDir         folder with the staged release files (required): the
;                  unzipped nudge-<version>-windows-x64.zip
;
; It mirrors the script installer of the zip (install.ps1, uninstall.ps1,
; toggle-autostart.ps1, firewall.ps1) and installs into the same folder, so
; it upgrades a script install in place:
;   - %LOCALAPPDATA%\Programs\Nudge, no administrator (PrivilegesRequired=lowest)
;   - Start Menu folder "Nudge" with "Nudge" -> nudge-background.exe
;   - autostart = Startup\Nudge.lnk -> nudge-background.exe, the same shortcut
;     toggle-autostart.ps1 creates (the tray's autostart toggle looks for that
;     file and runs {app}\toggle-autostart.ps1, so that script is installed)
;   - the install folder in the user PATH (HKCU\Environment), added once
;   - firewall rules "Nudge" (TCP 8765) and "Nudge mDNS" (UDP 5353), private
;     profile, same as firewall.ps1, but the elevated PowerShell gets the
;     commands inline (-EncodedCommand) and never runs a file from the
;     user-writable install folder (one UAC prompt, optional)
; install.ps1/uninstall.ps1 are not installed: running uninstall.ps1 would
; delete the folder behind Inno's back and leave a stale entry in Settings >
; Apps. Uninstall with Settings > Apps (or unins000.exe).

#ifndef AppVersion
  #error Define AppVersion, e.g. ISCC /DAppVersion=1.0.0
#endif
#ifndef SrcDir
  #error Define SrcDir (folder with nudge.exe, nudge-background.exe, ...), e.g. ISCC /DSrcDir=C:\t\build\pkg
#endif
#ifndef AppVersionNum
  #define AppVersionNum AppVersion
#endif

[Setup]
; Fixed forever: identifies the app for upgrades and the uninstall entry.
AppId={{E3D7174C-747F-45C5-B2D1-A2264785D367}
AppName=Nudge
AppVersion={#AppVersion}
AppPublisher=Scag95
AppPublisherURL=https://github.com/Scag95/Nudge
AppSupportURL=https://github.com/Scag95/Nudge/issues
AppUpdatesURL=https://github.com/Scag95/Nudge/releases
AppCopyright=Copyright (c) 2026 Scag95
VersionInfoVersion={#AppVersionNum}
VersionInfoProductVersion={#AppVersionNum}
VersionInfoProductTextVersion={#AppVersion}
VersionInfoCompany=Scag95
VersionInfoDescription=Instalador de Nudge
VersionInfoProductName=Nudge
; Per-user install without administrator, same folder as install.ps1.
PrivilegesRequired=lowest
DefaultDirName={localappdata}\Programs\Nudge
DisableDirPage=yes
DefaultGroupName=Nudge
DisableProgramGroupPage=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
WizardStyle=modern
LicenseFile=..\..\LICENSE
SetupIconFile={#SrcDir}\nudge.ico
UninstallDisplayName=Nudge
UninstallDisplayIcon={app}\nudge-background.exe
OutputDir=..\..\dist
OutputBaseFilename=nudge-setup-{#AppVersion}-windows-x64
Compression=lzma2
SolidCompression=yes
; The PATH lives in HKCU\Environment: tell Windows it changed.
ChangesEnvironment=yes
; Running Nudge is stopped in PrepareToInstall; the Restart Manager is a
; fallback for anything else holding a file. [Run] starts it again.
CloseApplications=yes
RestartApplications=no

[Languages]
Name: "es"; MessagesFile: "compiler:Languages\Spanish.isl"

[CustomMessages]
es.TaskGroup=Opciones:
es.TaskAutostart=Iniciar Nudge al iniciar sesión
es.TaskPath=Agregar nudge al PATH del usuario
es.TaskFirewall=Permitir Nudge en el Firewall de Windows (pide permiso de administrador)
es.RunNow=Iniciar Nudge ahora
es.StartMenuComment=Iniciar Nudge
es.AutostartComment=Nudge (servidor en segundo plano)
es.FirewallDeclined=No se agregaron las reglas del firewall (se canceló el permiso de administrador).%n%nSi el teléfono no encuentra la PC, volvé a ejecutar el instalador con la opción del firewall marcada.
es.FirewallFailed=No se pudieron agregar las reglas del firewall (código %1).%n%nSi el teléfono no encuentra la PC, volvé a ejecutar el instalador con la opción del firewall marcada.
es.FirewallRemoveFailed=No se borraron las reglas del firewall de Nudge.%n%nPara borrarlas a mano, en una consola de administrador:%n  netsh advfirewall firewall delete rule name="Nudge"%n  netsh advfirewall firewall delete rule name="Nudge mDNS"
es.AskRemoveData=¿Borrar también los datos en %1?%n%nAhí están los teléfonos emparejados, el certificado de la PC y el log. Si los borrás, cada teléfono tiene que emparejarse de nuevo.

[Tasks]
; All checked by default. Inno remembers the choice for the next upgrade.
Name: "autostart"; Description: "{cm:TaskAutostart}"; GroupDescription: "{cm:TaskGroup}"
Name: "path"; Description: "{cm:TaskPath}"; GroupDescription: "{cm:TaskGroup}"
Name: "firewall"; Description: "{cm:TaskFirewall}"; GroupDescription: "{cm:TaskGroup}"

[InstallDelete]
; Leftovers of the zip's script installer (upgrade in place): its extra
; Start Menu shortcuts and its own install/uninstall scripts.
Type: files; Name: "{userprograms}\Nudge\Nudge - autostart on-off.lnk"
Type: files; Name: "{userprograms}\Nudge\Uninstall Nudge.lnk"
Type: files; Name: "{app}\install.ps1"
Type: files; Name: "{app}\uninstall.ps1"

[Files]
Source: "{#SrcDir}\nudge.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SrcDir}\nudge-background.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SrcDir}\nudge.ico"; DestDir: "{app}"; Flags: ignoreversion
; Used by the tray's autostart toggle (same folder as the exe).
Source: "{#SrcDir}\toggle-autostart.ps1"; DestDir: "{app}"; Flags: ignoreversion
; Run elevated by this setup to add and remove the firewall rules.
Source: "{#SrcDir}\firewall.ps1"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SrcDir}\README-windows.txt"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
; Same name, target, icon and description as install.ps1's shortcut.
Name: "{userprograms}\Nudge\Nudge"; Filename: "{app}\nudge-background.exe"; WorkingDir: "{app}"; IconFilename: "{app}\nudge.ico"; IconIndex: 0; Comment: "{cm:StartMenuComment}"
; Same shortcut as `toggle-autostart.ps1 on` (the tray detects Nudge.lnk).
Name: "{userstartup}\Nudge"; Filename: "{app}\nudge-background.exe"; WorkingDir: "{app}"; IconFilename: "{app}\nudge.ico"; IconIndex: 0; Comment: "{cm:AutostartComment}"; Tasks: autostart

[Run]
; Not skipifsilent: a silent upgrade restarts the server it stopped.
Filename: "{app}\nudge-background.exe"; WorkingDir: "{app}"; Description: "{cm:RunNow}"; Flags: postinstall nowait

[UninstallDelete]
; The tray toggle may have created the autostart shortcut after install,
; so it is not always in the uninstall log.
Type: files; Name: "{userstartup}\Nudge.lnk"
Type: files; Name: "{userprograms}\Nudge\Nudge - autostart on-off.lnk"
Type: files; Name: "{userprograms}\Nudge\Uninstall Nudge.lnk"
Type: dirifempty; Name: "{userprograms}\Nudge"
Type: files; Name: "{app}\install.ps1"
Type: files; Name: "{app}\uninstall.ps1"
Type: dirifempty; Name: "{app}"

[Code]
const
  EnvKey = 'Environment';

{ ---------- small helpers ---------- }

function PowerShellExe: String;
begin
  Result := ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe');
end;

{ Runs a program hidden and waits; errors are only logged. }
procedure ExecHidden(const FileName, Params: String);
var
  RC: Integer;
begin
  if Exec(FileName, Params, '', SW_HIDE, ewWaitUntilTerminated, RC) then
    Log(Format('Exec %s %s -> %d', [FileName, Params, RC]))
  else
    Log(Format('Exec %s %s failed: %s', [FileName, Params, SysErrorMessage(RC)]));
end;

{ Stops Nudge so its files are not locked, like Stop-Nudge in install.ps1:
  every nudge-background.exe (it owns port 8765 and [Run] restarts it), and
  nudge.exe only when it runs from the install folder (never a dev build
  running elsewhere). Errors are ignored. }
procedure StopNudge;
var
  Dir: String;
begin
  ExecHidden(ExpandConstant('{sys}\taskkill.exe'), '/F /IM nudge-background.exe');
  Dir := ExpandConstant('{app}') + '\';
  { Single quotes are doubled for the PowerShell string literal. }
  StringChangeEx(Dir, '''', '''''', True);
  ExecHidden(PowerShellExe,
    '-NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "' +
    '$p = @(Get-Process -Name nudge -ErrorAction SilentlyContinue | Where-Object { $_.Path -and $_.Path.StartsWith(''' + Dir + ''', [StringComparison]::OrdinalIgnoreCase) }); ' +
    '$p | Stop-Process -Force -ErrorAction SilentlyContinue; ' +
    '$p | Wait-Process -Timeout 10 -ErrorAction SilentlyContinue"');
  { Give Windows a moment to release the file handles. }
  Sleep(500);
end;

{ ---------- user PATH (HKCU\Environment\Path, REG_EXPAND_SZ) ---------- }

{ Expands %VAR% references (unknown ones are kept as is). }
function ExpandEnvVars(const S: String): String;
var
  Rest, Name, Value: String;
  P: Integer;
begin
  Result := '';
  Rest := S;
  while True do
  begin
    P := Pos('%', Rest);
    if P = 0 then
    begin
      Result := Result + Rest;
      Break;
    end;
    Result := Result + Copy(Rest, 1, P - 1);
    Delete(Rest, 1, P);
    P := Pos('%', Rest);
    if P = 0 then
    begin
      Result := Result + '%' + Rest;
      Break;
    end;
    Name := Copy(Rest, 1, P - 1);
    Value := '';
    if Name <> '' then
      Value := GetEnv(Name);
    if Value <> '' then
    begin
      Result := Result + Value;
      Delete(Rest, 1, P);
    end
    else
    begin
      { Not a variable: keep "%Name" and retry from the second "%". }
      Result := Result + '%' + Name;
      Delete(Rest, 1, P - 1);
    end;
  end;
end;

{ Same test as Test-SamePath in install.ps1: case-insensitive, ignoring
  surrounding spaces and trailing backslashes, after expanding %VARS%. }
function SameDir(const Entry, Dir: String): Boolean;
var
  A, B: String;
begin
  A := Trim(ExpandEnvVars(Entry));
  B := Trim(Dir);
  while (Length(A) > 0) and (A[Length(A)] = '\') do
    Delete(A, Length(A), 1);
  while (Length(B) > 0) and (B[Length(B)] = '\') do
    Delete(B, Length(B), 1);
  Result := (A <> '') and (B <> '') and (CompareText(A, B) = 0);
end;

{ Returns PathValue without the entries equal to Dir (and without empty
  entries, like uninstall.ps1); Found tells whether Dir was there. }
function PathWithoutDir(const PathValue, Dir: String; var Found: Boolean): String;
var
  Rest, Entry: String;
  P: Integer;
begin
  Result := '';
  Found := False;
  Rest := PathValue;
  while Rest <> '' do
  begin
    P := Pos(';', Rest);
    if P = 0 then
    begin
      Entry := Rest;
      Rest := '';
    end
    else
    begin
      Entry := Copy(Rest, 1, P - 1);
      Delete(Rest, 1, P);
    end;
    if SameDir(Entry, Dir) then
      Found := True
    else if Entry <> '' then
    begin
      if Result <> '' then
        Result := Result + ';';
      Result := Result + Entry;
    end;
  end;
end;

function ReadUserPath: String;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, EnvKey, 'Path', Result) then
    Result := '';
end;

{ Adds Dir at the end of the user PATH unless it is already there. }
procedure AddToUserPath(const Dir: String);
var
  OldPath, NewPath: String;
  Found: Boolean;
begin
  OldPath := ReadUserPath;
  PathWithoutDir(OldPath, Dir, Found);
  if Found then
  begin
    Log('PATH: the folder is already in the user PATH');
    Exit;
  end;
  NewPath := OldPath;
  while (Length(NewPath) > 0) and (NewPath[Length(NewPath)] = ';') do
    Delete(NewPath, Length(NewPath), 1);
  if NewPath = '' then
    NewPath := Dir
  else
    NewPath := NewPath + ';' + Dir;
  if RegWriteExpandStringValue(HKEY_CURRENT_USER, EnvKey, 'Path', NewPath) then
    Log('PATH: added ' + Dir)
  else
    Log('PATH: could not write HKCU\Environment\Path');
end;

{ Removes exactly the Dir entries from the user PATH, nothing else. }
procedure RemoveFromUserPath(const Dir: String);
var
  NewPath: String;
  Found: Boolean;
begin
  NewPath := PathWithoutDir(ReadUserPath, Dir, Found);
  if not Found then
    Exit;
  if NewPath = '' then
    RegDeleteValue(HKEY_CURRENT_USER, EnvKey, 'Path')
  else
    RegWriteExpandStringValue(HKEY_CURRENT_USER, EnvKey, 'Path', NewPath);
  Log('PATH: removed ' + Dir);
end;

{ ---------- firewall (same rules as firewall.ps1, elevated, inline) ---------- }

function FirewallRuleExists(const Name: String): Boolean;
var
  RC: Integer;
begin
  { "show rule" works without administrator; exit code 0 = found. }
  Result := Exec(ExpandConstant('{sys}\netsh.exe'),
    'advfirewall firewall show rule name="' + Name + '"', '', SW_HIDE,
    ewWaitUntilTerminated, RC) and (RC = 0);
end;

{ Base64 of the UTF-16LE bytes of S, the format of powershell -EncodedCommand. }
function Utf16Base64(const S: String): String;
var
  Tbl: String;
  Bytes: array of Integer;
  I, N, B0, B1, B2: Integer;
begin
  Tbl := 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  N := Length(S) * 2;
  SetArrayLength(Bytes, N);
  for I := 1 to Length(S) do
  begin
    Bytes[(I - 1) * 2] := Ord(S[I]) and $FF;
    Bytes[(I - 1) * 2 + 1] := (Ord(S[I]) shr 8) and $FF;
  end;
  Result := '';
  I := 0;
  while I < N do
  begin
    B0 := Bytes[I];
    if I + 1 < N then B1 := Bytes[I + 1] else B1 := 0;
    if I + 2 < N then B2 := Bytes[I + 2] else B2 := 0;
    Result := Result + Tbl[(B0 shr 2) + 1]
      + Tbl[(((B0 and 3) shl 4) or (B1 shr 4)) + 1];
    if I + 1 < N then
      Result := Result + Tbl[(((B1 and 15) shl 2) or (B2 shr 6)) + 1]
    else
      Result := Result + '=';
    if I + 2 < N then
      Result := Result + Tbl[(B2 and 63) + 1]
    else
      Result := Result + '=';
    I := I + 3;
  end;
end;

{ Doubles single quotes so S can sit inside a PowerShell '...' literal. }
function PsQuote(const S: String): String;
begin
  Result := S;
  StringChangeEx(Result, '''', '''''', True);
end;

{ The PowerShell text that adds (Add = True, Prog = path of
  nudge-background.exe) or removes the Nudge rules; it mirrors firewall.ps1
  (names, ports, private profile, delete first so "add" never duplicates, exit
  3 when netsh fails or a rule survives the removal). The double quote that
  netsh needs comes from [char]34, so the text has no quoting to escape. }
function FirewallCommand(Add: Boolean; const Prog: String): String;
begin
  Result :=
    '$ErrorActionPreference=''Stop'';$q=[char]34;' +
    'function Invoke-Netsh([string]$a){' +
      '$i=New-Object System.Diagnostics.ProcessStartInfo;' +
      '$i.FileName=Join-Path $env:SystemRoot ''System32\netsh.exe'';' +
      '$i.Arguments=$a;$i.UseShellExecute=$false;$i.CreateNoWindow=$true;' +
      '$i.RedirectStandardOutput=$true;$i.RedirectStandardError=$true;' +
      '$p=[System.Diagnostics.Process]::Start($i);' +
      '[void]$p.StandardOutput.ReadToEnd();[void]$p.StandardError.ReadToEnd();' +
      '$p.WaitForExit();$p.ExitCode};' +
    'try{' +
    'foreach($n in ''Nudge'',''Nudge mDNS''){' +
      '[void](Invoke-Netsh (''advfirewall firewall delete rule name=''+$q+$n+$q))};';
  if Add then
    Result := Result +
      '$prog=''' + PsQuote(Prog) + ''';' +
      'foreach($a in ' +
      '(''advfirewall firewall add rule name=''+$q+''Nudge''+$q+'' dir=in action=allow protocol=TCP localport=8765 profile=private program=''+$q+$prog+$q+'' enable=yes''),' +
      '(''advfirewall firewall add rule name=''+$q+''Nudge mDNS''+$q+'' dir=in action=allow protocol=UDP localport=5353 profile=private program=''+$q+$prog+$q+'' enable=yes'')' +
      '){if((Invoke-Netsh $a) -ne 0){exit 3}};exit 0'
  else
    Result := Result +
      'foreach($n in ''Nudge'',''Nudge mDNS''){' +
        'if((Invoke-Netsh (''advfirewall firewall show rule name=''+$q+$n+$q)) -eq 0){exit 3}};exit 0';
  Result := Result + '}catch{exit 3}';
end;

{ Runs the firewall command in an elevated PowerShell (UAC prompt, hidden) and
  waits. Nothing from the install folder is executed: the command travels
  inline, so a file swapped there cannot be run as administrator. False when
  it could not start (e.g. the prompt was declined, RC = 1223); otherwise RC
  is the PowerShell exit code. }
function RunFirewallCommand(const Command: String; var RC: Integer): Boolean;
begin
  Result := ShellExec('runas', PowerShellExe,
    '-NoProfile -NonInteractive -ExecutionPolicy Bypass -EncodedCommand ' +
    Utf16Base64(Command), '', SW_HIDE, ewWaitUntilTerminated, RC);
end;

procedure AddFirewallRules;
var
  RC: Integer;
begin
  if FirewallRuleExists('Nudge') and FirewallRuleExists('Nudge mDNS') then
  begin
    Log('Firewall: the Nudge rules already exist');
    Exit;
  end;
  if not RunFirewallCommand(FirewallCommand(True,
    ExpandConstant('{app}\nudge-background.exe')), RC) then
  begin
    Log(Format('Firewall: not added (elevation declined or failed: %d %s)', [RC, SysErrorMessage(RC)]));
    SuppressibleMsgBox(CustomMessage('FirewallDeclined'), mbInformation, MB_OK, IDOK);
  end
  else if RC <> 0 then
  begin
    Log(Format('Firewall: the add command exited with %d', [RC]));
    SuppressibleMsgBox(FmtMessage(CustomMessage('FirewallFailed'), [IntToStr(RC)]), mbInformation, MB_OK, IDOK);
  end
  else
    Log('Firewall: rules added');
end;

{ Best effort, like uninstall.ps1: only asks for UAC when a rule exists.
  Runs no file from the install folder. }
procedure RemoveFirewallRules;
var
  RC: Integer;
begin
  if not (FirewallRuleExists('Nudge') or FirewallRuleExists('Nudge mDNS')) then
    Exit;
  if RunFirewallCommand(FirewallCommand(False, ''), RC) and (RC = 0) then
    Log('Firewall: rules removed')
  else
  begin
    Log(Format('Firewall: rules not removed (%d)', [RC]));
    SuppressibleMsgBox(CustomMessage('FirewallRemoveFailed'), mbInformation, MB_OK, IDOK);
  end;
end;

{ ---------- install ---------- }

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  StopNudge;
  Result := '';
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  AppDir: String;
begin
  if CurStep <> ssPostInstall then
    Exit;
  AppDir := ExpandConstant('{app}');

  { Each checkbox is the resulting state, like install.ps1 -NoAutostart:
    unchecked removes what an earlier install added. [Icons] creates the
    shortcut when the task is checked. }
  if not WizardIsTaskSelected('autostart') then
    DeleteFile(ExpandConstant('{userstartup}\Nudge.lnk'));

  if WizardIsTaskSelected('path') then
    AddToUserPath(AppDir)
  else
    RemoveFromUserPath(AppDir);

  { Unchecked leaves existing rules alone (removing them needs UAC too). }
  if WizardIsTaskSelected('firewall') then
    AddFirewallRules;
end;

{ ---------- uninstall ---------- }

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  DataDir: String;
begin
  case CurUninstallStep of
    usUninstall:
      begin
        StopNudge;
        RemoveFromUserPath(ExpandConstant('{app}'));
        RemoveFirewallRules;
      end;
    usPostUninstall:
      begin
        { Settings and paired phones: ask, default No (also when silent). }
        DataDir := ExpandConstant('{userappdata}\nudge');
        if DirExists(DataDir) then
        begin
          if SuppressibleMsgBox(FmtMessage(CustomMessage('AskRemoveData'), [DataDir]),
            mbConfirmation, MB_YESNO or MB_DEFBUTTON2, IDNO) = IDYES then
          begin
            if DelTree(DataDir, True, True, True) then
              Log('Data: removed ' + DataDir)
            else
              Log('Data: could not remove ' + DataDir);
          end
          else
            Log('Data: kept ' + DataDir);
        end;
      end;
  end;
end;
