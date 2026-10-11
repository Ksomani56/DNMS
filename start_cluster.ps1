# DNMS Enterprise Cluster Launcher
Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  DNMS Enterprise - Autonomous Distributed Monitoring Cluster" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. Clean up old processes
Write-Host "[1/3] Terminating any stale cluster processes in WSL..." -ForegroundColor Yellow
wsl -d Ubuntu -u root -- bash -c "killall dnms 2>/dev/null || true"
Start-Sleep -Seconds 1

# 2. Launch 4 nodes via supervisor
Write-Host "[2/3] Initializing 4-Node Cluster..." -ForegroundColor Green
Start-Process powershell -ArgumentList "-NoExit", "-Command", "wsl -d Ubuntu -u root -- bash /mnt/c/Users/Krushna/Documents/Projects/dnms/run_cluster.sh" -WindowStyle Minimized
Start-Sleep -Seconds 2

Write-Host ""
Write-Host "  Cluster successfully initialized:" -ForegroundColor Green
Write-Host "  - Node 1: http://localhost:12345 (127.0.0.1) [Spine-Leaf Fabric]" -ForegroundColor White
Write-Host "  - Node 2: http://localhost:12346 (127.0.0.2)" -ForegroundColor White
Write-Host "  - Node 3: http://localhost:12347 (127.0.0.3)" -ForegroundColor White
Write-Host "  - Node 4: http://localhost:12348 (127.0.0.4)" -ForegroundColor White
Write-Host ""
Write-Host "[3/3] Opening Enterprise Dashboard in browser..." -ForegroundColor Cyan

Start-Process "c:\Users\Krushna\Documents\Projects\dnms\visualization.html"

Write-Host ""
Write-Host "  Ready! To stop all nodes anytime, run: .\stop_cluster.ps1" -ForegroundColor Yellow
Write-Host "==========================================================" -ForegroundColor Cyan
