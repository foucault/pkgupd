RM= rm -vf
AR= ar
CC= gcc
CFLAGS= -std=c99 -fPIC -Wall -pedantic -O2
LDFLAGS= -lalpm
SOURCEFILES= goalpm.c
OBJECTFILES= $(patsubst %.c,%.o,$(SOURCEFILES))
LIBNAME= libgoalpm.so

.PHONY: all clean

all: $(LIBNAME) $(TARGET)

$(LIBNAME): $(OBJECTFILES)
	$(CC) $(CFLAGS) $(LDFLAGS) -shared $? -o $@

clean:
	$(RM) $(OBJECTFILES) $(LIBNAME)
