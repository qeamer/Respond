@echo off
title Respond Node

echo.
echo  RESPOND NODE
echo  =============
echo.

where go >nul 2>&1
if errorlevel 1 (
    echo [FEIL] Go ikke installert. Last ned: https://go.dev/dl/
    pause
    start https://go.dev/dl/
    exit /b 1
)

echo [OK] Go funnet
echo.
echo [1/3] go mod tidy...
go mod tidy
if errorlevel 1 (
    echo [FEIL] go mod tidy feilet
    pause
    exit /b 1
)

echo [2/3] Bygger respond.exe...
go build -ldflags="-s -w" -o respond.exe ./cmd/respond
if errorlevel 1 (
    echo [FEIL] Bygging feilet
    pause
    exit /b 1
)

echo [3/3] Starter Respond Node...
echo.
echo Apne nettleseren pa: http://localhost:8080
echo Stopp med Ctrl+C
echo.
respond.exe
