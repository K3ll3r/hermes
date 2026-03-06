# Hermes SSH login banner — shows pending notifications for headless sessions.
# Installed to C:\Program Files\Hermes\hermes-motd.ps1 by the MSI.
# Sourced from $PROFILE.AllUsersAllHosts via a guarded one-liner.

if (-not $env:SSH_CLIENT -and -not $env:SSH_CONNECTION) { return }
if ($Host.Name -ne 'ConsoleHost') { return }

$hermesExe = Join-Path $env:ProgramFiles 'Hermes\hermes.exe'
if (-not (Test-Path $hermesExe)) { return }

try {
    $raw = & $hermesExe inbox --json 2>$null
    if (-not $raw) { return }
    $json = if ($raw -is [array]) { $raw -join "`n" } else { $raw }
    $entries = ConvertFrom-Json -InputObject $json
    $count = @($entries).Count
    if ($count -eq 0) { return }

    Write-Host "`n-- Hermes: $count pending notification(s) --"
    $shown = 0
    foreach ($e in $entries) {
        if ($shown -ge 5) { break }
        $heading = $e.heading -replace '[\x00-\x1f\x7f-\x9f]', ''
        Write-Host "  * $heading"
        $shown++
    }
    if ($count -gt 5) { Write-Host "  ... and $($count - 5) more" }
    Write-Host "Run 'hermes.exe inbox' for details."
    Write-Host "----------------------------------------`n"
} catch {}
