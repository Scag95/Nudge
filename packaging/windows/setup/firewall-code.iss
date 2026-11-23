{ Firewall helpers shared by nudge.iss and setup\encoding-test.iss. Included
  inside a [Code] section. }

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

{ The PowerShell text that adds (Add = True, Prog = path of
  nudge-background.exe) or removes the Nudge rules; it mirrors firewall.ps1
  (names, ports, private profile, delete first so "add" never duplicates, exit
  3 when netsh fails or a rule survives the removal). The double quote that
  netsh needs comes from [char]34, so the text has no quoting to escape. The
  install path is never written as a quoted literal (PowerShell treats the
  typographic quotes U+2018..U+201B like ', and the path holds the user
  name): it travels as base64 of its UTF-16LE bytes, whose alphabet cannot
  break out of the literal, and is decoded inside PowerShell. }
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
      '$prog=[System.Text.Encoding]::Unicode.GetString([Convert]::FromBase64String(''' +
        Utf16Base64(Prog) + '''));' +
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
