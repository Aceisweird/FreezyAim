#!/bin/sh
# Rebuilds the icon and version details that Go links into FreezyAim.exe.
# Needs LLVM's windres. The result is committed, so ordinary builds don't need this.
set -e
cd "$(dirname "$0")"
"${WINDRES:-llvm-windres}" --target=x86_64-w64-mingw32 app.rc -o ../rsrc_windows_amd64.syso
