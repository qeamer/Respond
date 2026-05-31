@echo off
setlocal
title Respond Node

cd /d "%~dp0"

echo.
echo  =========================================
echo   RESPOND NODE
echo  =========================================
echo.

:: Kill any old respond.exe instance
taskkill /F /IM respond-node.exe >nul 2>&1
if not errorlevel 1 echo  [INFO] Stoppet gammel respond-node.exe
taskkill /F /IM Respond.exe >nul 2>&1

:: Check port 8080 availability
netstat -ano | findstr ":8080 " | findstr "LISTENING" >nul 2>&1
if not errorlevel 1 (
    echo  [ADVARSEL] Port 8080 er allerede i bruk av en annen prosess.
    echo  Finn og stopp den, eller endre porten i main.go
    netstat -ano | findstr ":8080 " | findstr "LISTENING"
    echo.
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
echo.
echo  ---------------------------------------------
echo  Apner klient: http://localhost:8080
echo  (IKKE localhost:3000 / file:// / 9090)
echo.
start "" "http://localhost:8080"

:: Run with auto-restart on crash
:loop
respond-node.exe
echo.
echo  [INFO] Respond.exe stoppet. Restart om 3 sek (Ctrl+C for a avbryte)...
timeout /t 3 /nobreak >nul
goto loop
