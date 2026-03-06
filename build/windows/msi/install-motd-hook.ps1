$line = 'if (Test-Path "$env:ProgramFiles\Hermes\hermes-motd.ps1") { . "$env:ProgramFiles\Hermes\hermes-motd.ps1" } # Hermes-MOTD'
$prof = $PROFILE.AllUsersAllHosts
if ((Test-Path $prof) -and (Select-String -Path $prof -Pattern 'Hermes-MOTD' -Quiet)) { exit 0 }
Add-Content -Path $prof -Value $line -Encoding UTF8
