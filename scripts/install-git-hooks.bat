@echo off
setlocal
git config core.hooksPath scripts\git-hooks
echo Git hooks installed (core.hooksPath -> scripts\git-hooks)
endlocal
