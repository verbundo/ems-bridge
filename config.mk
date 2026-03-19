# Build configuration — copy this file and adjust as needed.
# Changes here are local and should not be committed.

# Set to 1 to build with TIBCO EMS support.
# Windows: requires tibems.dll on PATH.
# Linux:   requires libtibemsssl64.so / libtibems64.so; set CGO_CFLAGS / CGO_LDFLAGS
#          if the TIBCO EMS SDK is not on the default search paths, e.g.:
#            export CGO_CFLAGS="-I/opt/tibco/ems/current/include"
#            export CGO_LDFLAGS="-L/opt/tibco/ems/current/lib -ltibems64"
TIBCO_EMS ?= 1
