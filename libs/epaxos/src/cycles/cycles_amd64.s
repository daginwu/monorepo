// Copyright 2016 David Terei.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// func Rdtsc() uint64
TEXT ·Rdtsc(SB),NOSPLIT,$0-8
    RDTSC
    SHLQ    $32, DX
    ADDQ    DX, AX
    MOVQ    AX, ret+0(FP)
    RET

// func Rdtscp() uint64
TEXT ·Rdtscp(SB),NOSPLIT,$0-8
    BYTE    $0x0F // RDTSCP
    BYTE    $0x01
    BYTE    $0xF9
    SHLQ    $32, DX
    ADDQ    DX, AX
    MOVQ    AX, ret+0(FP)
    CPUID
    RET
