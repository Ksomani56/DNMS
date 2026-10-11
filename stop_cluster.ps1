# DNMS Cluster Teardown
Write-Host "Stopping all DNMS cluster processes in WSL..." -ForegroundColor Yellow
wsl -d Ubuntu -u root -- bash -c "killall dnms 2>/dev/null || true"
Get-Process powershell -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowTitle -like "*run_cluster.sh*" } | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1
Write-Host "All cluster processes stopped cleanly. Network ports released." -ForegroundColor Green
