/* SPDX-License-Identifier: Apache-2.0 */
/* RSP-V0-021: no libc startup may observe stdin before the seek. */
.text
.global _start
_start:
    mov $8, %eax
    xor %edi, %edi
    xor %esi, %esi
    mov $2, %edx
    syscall
    mov %rax, %rdi
    mov $60, %eax
    syscall
.section .note.GNU-stack,"",@progbits
