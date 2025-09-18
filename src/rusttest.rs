use std::fmt::Error;
use std::num::ParseIntError;

pub trait Base {
    fn name(&self) -> &str;
    fn method1(&self, val: String) -> Result<String, ParseIntError>;
}

pub trait Derived: Base {
    fn method2(&self, val: i64) -> Result<String, ParseIntError>;
    fn method1(&self, val: String) -> Result<String, ParseIntError> {
        // Evaluate all arguments eagerly
        let val2: i64 = val.parse()?;
        self.method2(val2)
    }
}
pub struct DerivedExample1;

impl Base for DerivedExample1 {
    fn name(&self) -> &str {
        "DerivedExample1"
    }
    fn method1(&self, val: String) -> Result<String, ParseIntError> {
        Derived::method1(self, val)
    }
}

impl Derived for DerivedExample1 {
    fn method2(&self, val: i64) -> Result<String, ParseIntError> {
        Ok(format!("value={}", val * 2))
    }
}
fn convert_val(val: &str) -> Result<i64, ParseIntError> {
    val.parse::<i64>()
}

pub struct DerivedExample2;
impl Base for DerivedExample2 {
    fn name(&self) -> &str {
        "DerivedExample2"
    }
    fn method1(&self, val: String) -> Result<String, ParseIntError> {
        let val2 = convert_val(&val)?;
        Ok(format!("value={}", val2 * 2))
    }
}
