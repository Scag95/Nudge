; (UTF-8 with BOM: the vectors contain U+2019 and U+00E9 literals.)
; Test-only setup script (never shipped): checks the Utf16Base64 helper and
; the firewall command builder of nudge.iss against known vectors, with no UI.
; Exit code 0 = all checks pass, 1 = a check failed (see the /LOG file).
; Run it through scripts/test-windows-setup.sh.
[Setup]
AppName=Nudge encoding test
AppVersion=0
DefaultDirName={tmp}\nudge-encoding-test
Uninstallable=no
CreateAppDir=no
PrivilegesRequired=lowest
OutputBaseFilename=encoding-test

[Code]
procedure ExitProcess(ExitCode: Cardinal);
  external 'ExitProcess@kernel32.dll stdcall';

#include "firewall-code.iss"

var
  Failures: Integer;

procedure Check(const What: String; Ok: Boolean);
begin
  if Ok then
    Log('PASS: ' + What)
  else
  begin
    Log('FAIL: ' + What);
    Failures := Failures + 1;
  end;
end;

procedure CheckVector(const What, Input, Expected: String);
var
  Got: String;
begin
  Got := Utf16Base64(Input);
  Check(What, Got = Expected);
  if Got <> Expected then
    Log('  expected ' + Expected + #13#10 + '  got      ' + Got);
end;

{ Decodes standard base64 to the string whose UTF-16LE bytes it holds. }
function Utf16FromBase64(const B: String): String;
var
  Tbl: String;
  Bytes: array of Integer;
  I, N, V, Acc, Bits, P: Integer;
begin
  Tbl := 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  SetArrayLength(Bytes, Length(B));
  N := 0;
  Acc := 0;
  Bits := 0;
  for I := 1 to Length(B) do
  begin
    if B[I] = '=' then
      Break;
    P := Pos(B[I], Tbl);
    if P = 0 then
    begin
      Result := '<invalid base64>';
      Exit;
    end;
    Acc := (Acc shl 6) or (P - 1);
    Bits := Bits + 6;
    if Bits >= 8 then
    begin
      Bits := Bits - 8;
      V := (Acc shr Bits) and $FF;
      Acc := Acc and ((1 shl Bits) - 1);
      Bytes[N] := V;
      N := N + 1;
    end;
  end;
  Result := '';
  I := 0;
  while I + 1 < N do
  begin
    Result := Result + Chr(Bytes[I] or (Bytes[I + 1] shl 8));
    I := I + 2;
  end;
end;

procedure CheckFirewallCommand(const What, Prog: String);
var
  Cmd, B64: String;
  Enc: String;
begin
  Cmd := FirewallCommand(True, Prog);
  Enc := Utf16Base64(Cmd);
  Check(What + ': command round-trips through base64', Utf16FromBase64(Enc) = Cmd);
  B64 := Utf16Base64(Prog);
  Check(What + ': path travels only as base64',
    Pos('FromBase64String(''' + B64 + ''')', Cmd) > 0);
  Check(What + ': path is not embedded as text', (Pos(Prog, Cmd) = 0));
  Check(What + ': no typographic quote reaches the command',
    (Pos('‘', Cmd) = 0) and (Pos('’', Cmd) = 0));
end;

function InitializeSetup: Boolean;
var
  Ascii, OBrien, Jose: String;
begin
  Ascii := 'C:\Program Files\Nudge\nudge-background.exe';
  OBrien := 'C:\Users\O’Brien\AppData\Local\Programs\Nudge\nudge-background.exe';
  Jose := 'C:\Users\José\';

  CheckVector('ASCII path', Ascii,
    'QwA6AFwAUAByAG8AZwByAGEAbQAgAEYAaQBsAGUAcwBcAE4AdQBkAGcAZQBcAG4AdQBkAGcAZQAtAGIAYQBjAGsAZwByAG8AdQBuAGQALgBlAHgAZQA=');
  CheckVector('O' + 'Brien path (U+2019)', OBrien,
    'QwA6AFwAVQBzAGUAcgBzAFwATwAZIEIAcgBpAGUAbgBcAEEAcABwAEQAYQB0AGEAXABMAG8AYwBhAGwAXABQAHIAbwBnAHIAYQBtAHMAXABOAHUAZABnAGUAXABuAHUAZABnAGUALQBiAGEAYwBrAGcAcgBvAHUAbgBkAC4AZQB4AGUA');
  CheckVector('accented path', Jose, 'QwA6AFwAVQBzAGUAcgBzAFwASgBvAHMA6QBcAA==');
  CheckVector('empty string', '', '');

  CheckFirewallCommand('ASCII', Ascii);
  CheckFirewallCommand('O''Brien', OBrien);
  CheckFirewallCommand('accented', Jose);
  Check('remove command round-trips',
    Utf16FromBase64(Utf16Base64(FirewallCommand(False, ''))) = FirewallCommand(False, ''));

  if Failures = 0 then
    Log('ALL PASSED')
  else
    Log(IntToStr(Failures) + ' check(s) FAILED');
  { InitializeSetup returning False would give exit code 1 for a pass too:
    ExitProcess gives an exact 0 / 1 without any window. }
  if Failures = 0 then
    ExitProcess(0)
  else
    ExitProcess(1);
  Result := False;
end;
