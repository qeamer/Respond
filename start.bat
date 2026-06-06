@echo off
setlocal EnableDelayedExpansion
title Respond Node

cd /d "%~dp0"

echo.
echo  =========================================
echo   RESPOND NODE
echo  =========================================
echo.

:: Stopp våre egne prosesser som kan holde :8080
taskkill /F /IM respond-node.exe >nul 2>&1
if not errorlevel 1 echo  [INFO] Stoppet respond-node.exe
taskkill /F /IM Respond.exe >nul 2>&1
if not errorlevel 1 echo  [INFO] Stoppet Respond.exe (Wails desktop)
timeout /t 2 /nobreak >nul

call :port_pid
if defined PORT_PID (
    call :kill_pid !PORT_PID!
    timeout /t 1 /nobreak >nul
)

call :port_pid
if defined PORT_PID (
    echo  [ADVARSEL] Port 8080 er fortsatt i bruk ^(PID !PORT_PID!^).
    call :proc_name !PORT_PID!
    echo.
    echo  Hvis Respond desktop allerede kjører, trenger du IKKE start.bat.
    echo  Åpne bare: http://localhost:8080
    echo.
    echo  Ellers: lukk prosessen over ^(Task Manager^) og kjør start.bat på nytt.
    echo.
    start "" "http://localhost:8080"
    pause
    exit /b 1
)

where go >nul 2>&1
if errorlevel 1 (
    echo  [FEIL] Go er ikke installert. Last ned: https://go.dev/dl/
    pause & exit /b 1
)
echo  [BYGG] Bygger respond-node.exe (nettleser-prototype)...
go build -ldflags="-s -w" -o respond-node.exe ./cmd/respond-node
if errorlevel 1 (echo  [FEIL] go build feilet & pause & exit /b 1)
echo  [OK] Bygget.

echo.
echo  Starter Respond Node...
echo  Adresse:  http://localhost:8080
echo  Stopp:    Ctrl+C
echo.
echo  Desktop-app: wails dev  eller  wails build
echo  ^(Ikke kjør start.bat samtidig som Respond.exe — begge bruker port 8080^)
echo.
echo  ---------------------------------------------
echo  Apner klient: http://localhost:8080
echo  (IKKE localhost:3000 / file:// / 9090)
echo.
start "" "http://localhost:8080"

:loop
respond-node.exe
echo.
echo  [INFO] respond-node stoppet. Restart om 3 sek ^(Ctrl+C for å avbryte^)...
timeout /t 3 /nobreak >nul
goto loop

:port_pid
set PORT_PID=
for /f "tokens=5" %%p in ('netstat -ano ^| findstr ":8080 " ^| findstr "LISTENING"') do set PORT_PID=%%p
exit /b 0

:kill_pid
taskkill /F /PID %1 >nul 2>&1
if not errorlevel 1 echo  [INFO] Stoppet prosess PID %1 på port 8080
exit /b 0

:proc_name
for /f "tokens=1" %%n in ('tasklist /FI "PID eq %1" /FO LIST ^| findstr /I "Image Name"') do echo  Prosess: %%n
exit /b 0
