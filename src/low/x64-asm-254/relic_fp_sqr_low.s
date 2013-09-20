/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file
 * for contact information.
 *
 * RELIC is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 2.1 of the License, or (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

#include "relic_fp_low.h"

/**
 * @file
 *
 * Implementation of low-level prime field multiplication.
 *
 * @version $Id$
 * @ingroup fp
 */

.text
.global fp_sqrn_low

.macro COMBA_STEP a, b
	movq	\a, %rax
	mulq	\b
	addq	%rax,%rax
	adcq	%rdx,%rdx
	adcq	$0, %r10
	addq	%rax,%r8
	adcq	%rdx,%r9
	adcq	$0,%r10
.endm

.macro COMBA_FINAL i
	movq	8*\i(%rsi), %rax
	mulq	8*\i(%rsi)
	addq	%rax,%r8
	adcq	%rdx,%r9
	adcq	$0,%r10
.endm

.macro SQRN_STEP i, j
	COMBA_STEP 8*\i(%rsi), 8*\j(%rsi)
.endm
/*
 * Function: fp_sqrn_low
 * Inputs: rdi = c, rsi = a
 * Output: rax
 */
fp_sqrn_low:
  xorq %r10,%r10
  movq 0(%rsi),%rax
  mulq 8(%rsi)
  addq %rax,%rax
  movq %rax,%r8
  adcq %rdx,%rdx
  movq %rdx,%r9
  adcq $0,%r10

  movq 0(%rsi),%rax
  mulq %rax
  movq %rax,0(%rdi)
  addq %rdx,%r8
  movq %r8,8(%rdi)
  adcq $0,%r9

  xorq %rcx,%rcx
  movq 0(%rsi),%rax
  mulq 16(%rsi)
  addq %rax,%rax
  movq %rax,%r8
  adcq %rdx,%rdx
  movq %rdx,%r11
  adcq $0,%rcx

  movq 8(%rsi),%rax
  mulq 8(%rsi)
  addq %rax,%r8
  adcq %rdx,%r11
  adcq $0,%rcx

  movq 0(%rsi),%rax
  mulq 24(%rsi)
  addq %r9,%r8
  movq %r8,16(%rdi)
  adcq %r10,%r11
  adcq $0,%rcx
  movq %rax,%r8
  movq %rdx,%r10
  
  xorq %r9,%r9 
  movq 8(%rsi),%rax
  mulq 16(%rsi)
  addq %rax,%r8
  adcq %rdx,%r10
  adcq $0,%r9
  addq %r8,%r8
  adcq %r10,%r10
  adcq %r9,%r9

  movq 8(%rsi),%rax
  mulq 24(%rsi)
  addq %r11,%r8
  movq %r8,24(%rdi)
  adcq %rcx,%r10
  adcq $0,%r9
  xorq %rcx,%rcx
  addq %rax,%rax
  movq %rax,%r8
  adcq %rdx,%rdx
  movq %rdx,%r11
  adcq $0,%rcx

  movq 16(%rsi),%rax
  mulq 16(%rsi)
  addq %r10,%r8
  adcq %r11,%r9
  adcq $0,%rcx
  addq %rax,%r8
  movq %r8,32(%rdi)
  adcq %rdx,%r9
  adcq $0,%rcx

  xorq %r11,%r11
  movq 16(%rsi),%rax
  mulq 24(%rsi)
  addq %rax,%rax
  adcq %rdx,%rdx
  adcq $0,%r11
  addq %rax,%r9
  movq %r9,40(%rdi)
  adcq %rdx,%rcx
  adcq $0,%r11

  movq 24(%rsi),%rax
  mulq %rax
  addq %rax,%rcx
  movq %rcx,48(%rdi)
  adcq %rdx,%r11
  movq %r11,56(%rdi)
ret

/*
	push	%r13
	push	%r14

	movq	0(%rsi), %rcx
	movq	%rcx, %rax
	mulq	%rax
	movq	%rax,%r8
	movq	%rdx,%r9
	xorq	%r10, %r10
	//COMBA_FINAL 0
	movq	%r8, 0(%rdi)
	movq	%r9, %r8
	movq	%r10,%r9

	movq	8(%rsi), %r11
	movq	%r11, %rax
	mulq	%rcx
	addq	%rax,%rax
	adcq	%rdx,%rdx
	adcq	$0, %r10
	addq	%rax,%r8
	adcq	%rdx,%r9
	adcq	$0,%r10
	//SQRN_STEP 0, 1
	movq	%r8, 8(%rdi)
	movq	%r9, %r8
	movq	%r10,%r9
	xorq	%r10, %r10

	movq	16(%rsi), %r13
	movq	%rcx, %rax
	mulq	%r13
	addq	%rax,%rax
	adcq	%rdx,%rdx
	adcq	$0, %r10
	addq	%rax,%r8
	adcq	%rdx,%r9
	adcq	$0,%r10
	//SQRN_STEP 0, 2
	movq	%r11, %rax
	mulq	%r11
	addq	%rax,%r8
	adcq	%rdx,%r9
	adcq	$0,%r10
	//COMBA_FINAL 1
	movq	%r8, 16(%rdi)
	movq	%r9, %r8
	movq	%r10,%r9
	xorq	%r10, %r10

	movq	24(%rsi), %r14
	movq	%rcx, %rax
	mulq	%r14
	addq	%rax,%rax
	adcq	%rdx,%rdx
	adcq	$0, %r10
	addq	%rax,%r8
	adcq	%rdx,%r9
	adcq	$0,%r10
	//SQRN_STEP 0, 3
	movq	%r11, %rax
	mulq	%r13
	addq	%rax,%rax
	adcq	%rdx,%rdx
	adcq	$0, %r10
	addq	%rax,%r8
	adcq	%rdx,%r9
	adcq	$0,%r10
	//SQRN_STEP 1, 2
	movq	%r8, 24(%rdi)
	movq	%r9, %r8
	movq	%r10,%r9
	xorq	%r10, %r10

	movq	%r11, %rax
	mulq	%r14
	addq	%rax,%rax
	adcq	%rdx,%rdx
	adcq	$0, %r10
	addq	%rax,%r8
	adcq	%rdx,%r9
	adcq	$0,%r10
	//SQRN_STEP 1, 3
	movq	%r13, %rax
	mulq	%r13
	addq	%rax,%r8
	adcq	%rdx,%r9
	adcq	$0,%r10
	//COMBA_FINAL 2
	movq	%r8, 32(%rdi)
	movq	%r9, %r8
	movq	%r10,%r9
	xorq	%r10, %r10

	movq	%r13, %rax
	mulq	%r14
	addq	%rax,%rax
	adcq	%rdx,%rdx
	adcq	$0, %r10
	addq	%rax,%r8
	adcq	%rdx,%r9
	adcq	$0,%r10
	//SQRN_STEP 2, 3
	movq	%r8, 40(%rdi)
	movq	%r9, %r8
	movq	%r10,%r9
	xorq	%r10, %r10

	movq	%r14, %rax
	mulq	%r14
	addq	%rax,%r8
	adcq	%rdx,%r9
	//COMBA_FINAL 3
	movq	%r8, 48(%rdi)
	movq	%r9, 56(%rdi)

	pop		%r14
	pop		%r13
	ret
*/
