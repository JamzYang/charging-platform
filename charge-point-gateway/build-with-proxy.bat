@echo off
echo Setting temporary proxy for this session...
set HTTP_PROXY=http://192.168.68.16:7890
set HTTPS_PROXY=http://192.168.68.16:7890

echo Starting Docker Compose build...
docker-compose -f test/docker-compose.test.yml up --build -d

echo.
echo Build command finished.
pause