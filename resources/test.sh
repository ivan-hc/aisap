#!/bin/sh
# Make sure everything runs
# Will eventually make it more in-depth

./*.shImg --help
[ $? -ne 0 ] && exit 1

./*.AppImage --help
[ $? -ne 0 ] && exit 1
