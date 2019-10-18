set PROJECT_HOME="H:\programs\ruis\go\go-sdl-ffplay"

set CGO_CFLAGS=-I%PROJECT_HOME%/include
set CGO_LDFLAGS=-L%PROJECT_HOME%/bin -lSDL2 -lavcodec-58 -lavdevice-58 -lavfilter-7 -lavformat-58 -lavutil-56 -lpostproc-55 -lswresample-3 -lswscale-5
go build -o bin/goffplay.exe main.go