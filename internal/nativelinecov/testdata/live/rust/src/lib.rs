// SPDX-License-Identifier: Apache-2.0
pub fn value(x: i32) -> i32 {
    if x == 1 {
        return 11;
    }
    if x == 2 {
        return 22;
    }
    33
}
#[cfg(test)]
mod tests {
    #[test] fn alpha() { assert_eq!(super::value(1), 11); }
    #[test] fn beta() { assert_eq!(super::value(2), 22); }
}
