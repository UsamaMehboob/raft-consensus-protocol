#!/usr/bin/env bash

count=0
while true; do
    let "count+=1"
    echo "count=$count Usama"
    go test -run TestFigure8Unreliable2C > longfail
    if [ $? -ne 0 ]; then
        break
    fi
    cp longfail longfail-old
    rm longfail
    sleep 1
done