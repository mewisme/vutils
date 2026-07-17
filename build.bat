@echo off
go run github.com/akavel/rsrc@v0.10.2 -ico icon.ico -arch amd64 -o rsrc_windows_amd64.syso
if errorlevel 1 exit /b 1
go build -ldflags="-H windowsgui -X github.com/mewisme/vutils/internal/version.Version=dev" -o build\bin\vutils.exe .
if errorlevel 1 exit /b 1
echo built build\bin\vutils.exe
