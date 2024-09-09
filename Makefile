LDFLAGS := -ldflags="-s -w"
SOURCES := main.go
OUT := gochat

ifneq (,$(findstring NT,$(shell uname)))
	OUT := $(OUT).exe
endif
DATE := $(shell date -u +%Y%m%d)

default: $(OUT)
$(OUT): $(SOURCES)
	go build $(LDFLAGS) -o $(OUT) $(SOURCES)
