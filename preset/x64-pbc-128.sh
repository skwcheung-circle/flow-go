#!/bin/bash 
cmake -DWORD=64 -DSEED=ZERO -DSHLIB=OFF -DSTBIN=ON -DTIMER=CYCLE -DWITH="MD;DV;BN;FP;EP;PP" -DCHECK=off -DVERBS=off -DARITH=x64-asm-254 -DFP_PRIME=254 -DFP_METHD="INTEG;INTEG;INTEG;MONTY;LOWER;SLIDE" -DCOMP="-O3 -funroll-loops -fomit-frame-pointer -finline-small-functions -march=native -mtune=native" -DFP_PMERS=off -DFP_QNRES=on -DPP_METHD="INTEG;INTEG;LAZYR;OATEP" -DBN_PRECI=256 $1
