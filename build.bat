@echo off
go build -ldflags="-H windowsgui -X github.com/mewisme/vutils/internal/version.Version=dev" -o build\bin\vutils.exe .
if errorlevel 1 exit /b 1
echo built build\bin\vutils.exe
