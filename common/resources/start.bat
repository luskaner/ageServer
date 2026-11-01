@echo off
chcp 65001 >nul 2>&1
cd /d "%~dp0"
%*
if %ERRORLEVEL%==0 (
    echo Program finished successfully, closing in 10 seconds...
    timeout /t 10
) else (
    echo Program finished with errors...
    pause
)