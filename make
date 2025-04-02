# Makefile for 3270Connect builds

APP_NAME = 3270Connect
VERSION = 1.3.3

.PHONY: all windows windows_gui linux clean

all: windows windows_gui linux

windows:
	GOOS=windows GOARCH=amd64 go build -o $(APP_NAME).exe -ldflags="-H=windowsgui=false" -v

windows_gui:
	GOOS=windows GOARCH=amd64 go build -o $(APP_NAME)_gui.exe -ldflags="-H=windowsgui" -v

linux:
	GOOS=linux GOARCH=amd64 go build -o $(APP_NAME)_linux -v

clean:
	rm -f $(APP_NAME).exe $(APP_NAME)_gui.exe $(APP_NAME)_linux
