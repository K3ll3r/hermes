$prof = $PROFILE.AllUsersAllHosts
if (-not (Test-Path $prof)) { exit 0 }
$lines = Get-Content -Path $prof -Encoding UTF8 | Where-Object { $_ -notmatch '^\s*if \(Test-Path .+hermes-motd\.ps1.+# Hermes-MOTD\s*$' }
Set-Content -Path $prof -Value $lines -Encoding UTF8
