//! WDL data types system
//!
//! WDL has both atomic types such as `Int`, `Boolean`, and `String`; and
//! parametric types like `Array[String]` and `Map[String,Array[Array[Float]]]`.
//! Each type is represented by an immutable instance of a Rust enum.
//!
//! Type coercion rules:
//! 1. `Int` coerces to `Float`
//! 2. `Boolean`, `Int`, `Float`, and `File` coerce to `String`
//! 3. `String` coerces to `File`, `Int`, and `Float`
//! 4. `Array[T]` coerces to `String` provided `T` does as well
//! 5. `T` coerces to `T?` but the reverse is not true in general
//! 6. `Array[T]+` coerces to `Array[T]` but the reverse is not true in general

use crate::error::{SourcePosition, WdlError};
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet};
use std::fmt;

/// The base type for all WDL types.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum Type {
    /// A symbolic type which coerces to any other type
    Any { optional: bool },

    /// Boolean type (true/false)
    Boolean { optional: bool },

    /// Integer type  
    Int { optional: bool },

    /// Floating point type
    Float { optional: bool },

    /// String type
    String { optional: bool },

    /// File type (represents a filesystem path)
    File { optional: bool },

    /// Directory type (represents a directory path)
    Directory { optional: bool },

    /// Array type, parameterized by item type
    Array {
        item_type: Box<Type>,
        optional: bool,
        nonempty: bool,
    },

    /// Map type, parameterized by key and value types
    Map {
        key_type: Box<Type>,
        value_type: Box<Type>,
        optional: bool,
        literal_keys: Option<HashSet<String>>,
    },

    /// Pair type, parameterized by left and right types
    Pair {
        left_type: Box<Type>,
        right_type: Box<Type>,
        optional: bool,
    },

    /// Instance of a struct type
    StructInstance {
        type_name: String,
        members: Option<HashMap<String, Type>>,
        optional: bool,
    },

    /// Object type (transient, for struct initialization)
    Object {
        members: HashMap<String, Type>,
        is_call_output: bool,
    },
}

impl Type {
    /// Create a new Any type.
    pub fn any() -> Self {
        Type::Any { optional: false }
    }

    /// Create a new None type (optional Any).
    pub fn none() -> Self {
        Type::Any { optional: true }
    }

    /// Create a new Boolean type.
    pub fn boolean(optional: bool) -> Self {
        Type::Boolean { optional }
    }

    /// Create a new Int type.
    pub fn int(optional: bool) -> Self {
        Type::Int { optional }
    }

    /// Create a new Float type.
    pub fn float(optional: bool) -> Self {
        Type::Float { optional }
    }

    /// Create a new String type.
    pub fn string(optional: bool) -> Self {
        Type::String { optional }
    }

    /// Create a new File type.
    pub fn file(optional: bool) -> Self {
        Type::File { optional }
    }

    /// Create a new Directory type.
    pub fn directory(optional: bool) -> Self {
        Type::Directory { optional }
    }

    /// Create a new Array type.
    pub fn array(item_type: Type, optional: bool, nonempty: bool) -> Self {
        Type::Array {
            item_type: Box::new(item_type),
            optional,
            nonempty,
        }
    }

    /// Create a new Map type.
    pub fn map(key_type: Type, value_type: Type, optional: bool) -> Self {
        Type::Map {
            key_type: Box::new(key_type),
            value_type: Box::new(value_type),
            optional,
            literal_keys: None,
        }
    }

    /// Create a new Map type with literal keys.
    pub fn map_with_keys(
        key_type: Type,
        value_type: Type,
        optional: bool,
        literal_keys: HashSet<String>,
    ) -> Self {
        Type::Map {
            key_type: Box::new(key_type),
            value_type: Box::new(value_type),
            optional,
            literal_keys: Some(literal_keys),
        }
    }

    /// Create a new Pair type.
    pub fn pair(left_type: Type, right_type: Type, optional: bool) -> Self {
        Type::Pair {
            left_type: Box::new(left_type),
            right_type: Box::new(right_type),
            optional,
        }
    }

    /// Create a new StructInstance type.
    pub fn struct_instance(type_name: String, optional: bool) -> Self {
        Type::StructInstance {
            type_name,
            members: None,
            optional,
        }
    }

    /// Create a new Object type.
    pub fn object(members: HashMap<String, Type>) -> Self {
        Type::Object {
            members,
            is_call_output: false,
        }
    }

    /// Create a new Object type for call outputs.
    pub fn object_call_output(members: HashMap<String, Type>) -> Self {
        Type::Object {
            members,
            is_call_output: true,
        }
    }

    /// Check if this type is optional.
    pub fn is_optional(&self) -> bool {
        match self {
            Type::Any { optional } => *optional,
            Type::Boolean { optional } => *optional,
            Type::Int { optional } => *optional,
            Type::Float { optional } => *optional,
            Type::String { optional } => *optional,
            Type::File { optional } => *optional,
            Type::Directory { optional } => *optional,
            Type::Array { optional, .. } => *optional,
            Type::Map { optional, .. } => *optional,
            Type::Pair { optional, .. } => *optional,
            Type::StructInstance { optional, .. } => *optional,
            Type::Object { .. } => false,
        }
    }

    /// Check if this is a nonempty array type.
    pub fn is_nonempty(&self) -> bool {
        match self {
            Type::Array { nonempty, .. } => *nonempty,
            _ => false,
        }
    }

    /// Create a copy of this type with a different optional setting.
    pub fn with_optional(mut self, optional: bool) -> Self {
        match &mut self {
            Type::Any { optional: opt } => *opt = optional,
            Type::Boolean { optional: opt } => *opt = optional,
            Type::Int { optional: opt } => *opt = optional,
            Type::Float { optional: opt } => *opt = optional,
            Type::String { optional: opt } => *opt = optional,
            Type::File { optional: opt } => *opt = optional,
            Type::Directory { optional: opt } => *opt = optional,
            Type::Array { optional: opt, .. } => *opt = optional,
            Type::Map { optional: opt, .. } => *opt = optional,
            Type::Pair { optional: opt, .. } => *opt = optional,
            Type::StructInstance { optional: opt, .. } => *opt = optional,
            Type::Object { .. } => {} // Object types don't have optional
        }
        self
    }

    /// Create a copy of an Array type with a different nonempty setting.
    pub fn with_nonempty(mut self, nonempty: bool) -> Result<Self, WdlError> {
        match &mut self {
            Type::Array { nonempty: ne, .. } => {
                *ne = nonempty;
                Ok(self)
            }
            _ => Err(WdlError::validation_error(
                SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                "Cannot set nonempty on non-array type".to_string(),
            )),
        }
    }

    /// Get the parameter types of this type.
    pub fn parameters(&self) -> Vec<&Type> {
        match self {
            Type::Array { item_type, .. } => vec![item_type],
            Type::Map {
                key_type,
                value_type,
                ..
            } => vec![key_type, value_type],
            Type::Pair {
                left_type,
                right_type,
                ..
            } => vec![left_type, right_type],
            Type::StructInstance {
                members: Some(m), ..
            } => m.values().collect(),
            Type::Object {
                members,
                is_call_output: false,
            } => members.values().collect(),
            _ => vec![],
        }
    }

    /// Check if this type can be coerced to another type.
    pub fn coerces(&self, rhs: &Type, check_quant: bool) -> bool {
        self.check_coercion(rhs, check_quant).is_ok()
    }

    /// Check type coercion, returning detailed error if impossible.
    pub fn check_coercion(&self, rhs: &Type, check_quant: bool) -> Result<(), WdlError> {
        // Handle Any type - coerces to/from anything
        if matches!(self, Type::Any { .. }) || matches!(rhs, Type::Any { .. }) {
            return self.check_optional(rhs, check_quant);
        }

        // Handle array promotion: T -> Array[T] when not checking quantifiers
        if !check_quant {
            if let Type::Array { item_type, .. } = rhs {
                if self.coerces(item_type, check_quant) {
                    return Ok(());
                }
            }
        }

        match (self, rhs) {
            // Same type variants
            (Type::Boolean { .. }, Type::Boolean { .. })
            | (Type::Int { .. }, Type::Int { .. })
            | (Type::Float { .. }, Type::Float { .. })
            | (Type::String { .. }, Type::String { .. })
            | (Type::File { .. }, Type::File { .. })
            | (Type::Directory { .. }, Type::Directory { .. }) => {
                self.check_optional(rhs, check_quant)
            }

            // Int coerces to Float
            (Type::Int { .. }, Type::Float { .. }) => self.check_optional(rhs, check_quant),

            // Boolean, Int, Float, File coerce to String
            (
                Type::Boolean { .. } | Type::Int { .. } | Type::Float { .. } | Type::File { .. },
                Type::String { .. },
            ) => self.check_optional(rhs, check_quant),

            // String coerces to File, Directory, Int, Float
            (
                Type::String { .. },
                Type::File { .. } | Type::Directory { .. } | Type::Int { .. } | Type::Float { .. },
            ) => self.check_optional(rhs, check_quant),

            // Array type coercion
            (
                Type::Array {
                    item_type: lhs_item,
                    ..
                },
                Type::Array {
                    item_type: rhs_item,
                    ..
                },
            ) => {
                lhs_item.check_coercion(rhs_item, check_quant)?;
                self.check_optional(rhs, check_quant)
            }

            // Array coerces to String if item type does
            (Type::Array { item_type, .. }, Type::String { .. }) => {
                item_type.check_coercion(&Type::string(false), check_quant)?;
                self.check_optional(rhs, check_quant)
            }

            // Map type coercion
            (
                Type::Map {
                    key_type: lhs_k,
                    value_type: lhs_v,
                    ..
                },
                Type::Map {
                    key_type: rhs_k,
                    value_type: rhs_v,
                    ..
                },
            ) => {
                lhs_k.check_coercion(rhs_k, check_quant)?;
                lhs_v.check_coercion(rhs_v, check_quant)?;
                self.check_optional(rhs, check_quant)
            }

            // Map with literal keys to struct
            (
                Type::Map {
                    key_type: _,
                    value_type,
                    literal_keys: Some(keys),
                    ..
                },
                Type::StructInstance {
                    members: Some(struct_members),
                    ..
                },
            ) => self.check_struct_members(keys, value_type, struct_members, check_quant),

            // Pair type coercion
            (
                Type::Pair {
                    left_type: lhs_l,
                    right_type: lhs_r,
                    ..
                },
                Type::Pair {
                    left_type: rhs_l,
                    right_type: rhs_r,
                    ..
                },
            ) => {
                lhs_l.check_coercion(rhs_l, check_quant)?;
                lhs_r.check_coercion(rhs_r, check_quant)?;
                self.check_optional(rhs, check_quant)
            }

            // StructInstance coercion
            (
                Type::StructInstance {
                    type_name: lhs_name,
                    members: lhs_members,
                    ..
                },
                Type::StructInstance {
                    type_name: rhs_name,
                    members: rhs_members,
                    ..
                },
            ) => {
                // Check if same struct type by comparing type IDs
                if let (Some(lhs_m), Some(rhs_m)) = (lhs_members, rhs_members) {
                    if struct_type_id(lhs_m) != struct_type_id(rhs_m) {
                        return Err(WdlError::static_type_mismatch(
                            SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                            rhs_name.clone(),
                            lhs_name.clone(),
                            "".to_string(),
                        ));
                    }
                }
                self.check_optional(rhs, check_quant)
            }

            // Object to struct coercion
            (
                Type::Object {
                    members,
                    is_call_output: false,
                },
                Type::StructInstance {
                    members: Some(struct_members),
                    ..
                },
            ) => self.check_struct_members(
                &members.keys().cloned().collect(),
                &Type::any(),
                struct_members,
                check_quant,
            ),

            // Object to Map
            (
                Type::Object {
                    members,
                    is_call_output: false,
                },
                Type::Map {
                    key_type,
                    value_type,
                    ..
                },
            ) => {
                // Member names must coerce to map key type
                Type::string(false).check_coercion(key_type, check_quant)?;
                // Each member type must coerce to map value type
                for member_type in members.values() {
                    member_type.check_coercion(value_type, check_quant)?;
                }
                Ok(())
            }

            // Object to Object coercion
            (
                Type::Object {
                    members: lhs_members,
                    is_call_output: false,
                },
                Type::Object {
                    members: rhs_members,
                    is_call_output: false,
                },
            ) => {
                // If RHS (target) has no specific member types (from type declaration), 
                // any Object can coerce to it
                if rhs_members.is_empty() {
                    return Ok(());
                }
                
                // If RHS has specific member types, check that LHS provides compatible types
                for (rhs_key, rhs_type) in rhs_members {
                    if let Some(lhs_type) = lhs_members.get(rhs_key) {
                        lhs_type.check_coercion(rhs_type, check_quant)?;
                    } else {
                        return Err(WdlError::validation_error(
                            SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                            format!("Object missing required member: {}", rhs_key),
                        ));
                    }
                }
                Ok(())
            }

            // Map to Object (reverse coercion for WDL compatibility)
            (
                Type::Map {
                    key_type,
                    value_type,
                    ..
                },
                Type::Object {
                    members,
                    is_call_output: false,
                },
            ) => {
                // Map keys must be strings
                key_type.check_coercion(&Type::string(false), check_quant)?;
                // If Object has specific member types, check value type coerces to each
                if !members.is_empty() {
                    for member_type in members.values() {
                        value_type.check_coercion(member_type, check_quant)?;
                    }
                } else {
                    // Empty Object (from parser) accepts any Map[String, T]
                }
                Ok(())
            }

            // Default: types don't coerce
            _ => Err(WdlError::static_type_mismatch(
                SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                rhs.to_string(),
                self.to_string(),
                "".to_string(),
            )),
        }
    }

    /// Check optional quantifier compatibility.
    fn check_optional(&self, rhs: &Type, check_quant: bool) -> Result<(), WdlError> {
        if check_quant
            && self.is_optional()
            && !rhs.is_optional()
            && !matches!(rhs, Type::Any { .. })
        {
            Err(WdlError::static_type_mismatch(
                SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                rhs.to_string(),
                self.to_string(),
                "Cannot coerce optional type to non-optional".to_string(),
            ))
        } else {
            Ok(())
        }
    }

    /// Check struct member coercion.
    fn check_struct_members(
        &self,
        literal_keys: &HashSet<String>,
        value_type: &Type,
        struct_members: &HashMap<String, Type>,
        check_quant: bool,
    ) -> Result<(), WdlError> {
        let struct_keys: HashSet<String> = struct_members.keys().cloned().collect();
        let missing_keys: Vec<String> = struct_keys
            .difference(literal_keys)
            .filter(|k| !struct_members[*k].is_optional())
            .cloned()
            .collect();

        if !missing_keys.is_empty() {
            return Err(WdlError::validation_error(
                SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                format!(
                    "Missing non-optional struct members: {}",
                    missing_keys.join(", ")
                ),
            ));
        }

        for key in literal_keys.intersection(&struct_keys) {
            value_type.check_coercion(&struct_members[key], check_quant)?;
        }

        Ok(())
    }

    /// Check if values of these types can be compared for equality.
    pub fn equatable(&self, rhs: &Type, compound: bool) -> bool {
        match (self, rhs) {
            (Type::Any { .. }, _) | (_, Type::Any { .. }) => true,
            (Type::Object { .. }, _) | (_, Type::Object { .. }) => false,

            // Int/Float can be equated even in compound types (arrays, maps, etc)
            (Type::Int { .. }, Type::Float { .. }) | (Type::Float { .. }, Type::Int { .. }) => true,

            // File and String are equatable per WDL spec: "File is substituted as if it were a String"
            (Type::File { .. }, Type::String { .. }) | (Type::String { .. }, Type::File { .. }) => {
                true
            }

            // Same type variants are equatable
            (Type::Boolean { .. }, Type::Boolean { .. })
            | (Type::Int { .. }, Type::Int { .. })
            | (Type::Float { .. }, Type::Float { .. })
            | (Type::String { .. }, Type::String { .. })
            | (Type::File { .. }, Type::File { .. })
            | (Type::Directory { .. }, Type::Directory { .. }) => true,

            // Compound types recurse
            (Type::Array { item_type: lhs, .. }, Type::Array { item_type: rhs, .. }) => {
                lhs.equatable(rhs, true)
            }
            (
                Type::Map {
                    key_type: lhs_k,
                    value_type: lhs_v,
                    ..
                },
                Type::Map {
                    key_type: rhs_k,
                    value_type: rhs_v,
                    ..
                },
            ) => lhs_k.equatable(rhs_k, true) && lhs_v.equatable(rhs_v, true),
            (
                Type::Pair {
                    left_type: lhs_l,
                    right_type: lhs_r,
                    ..
                },
                Type::Pair {
                    left_type: rhs_l,
                    right_type: rhs_r,
                    ..
                },
            ) => lhs_l.equatable(rhs_l, true) && lhs_r.equatable(rhs_r, true),
            (
                Type::StructInstance {
                    members: Some(lhs), ..
                },
                Type::StructInstance {
                    members: Some(rhs), ..
                },
            ) => struct_type_id(lhs) == struct_type_id(rhs),

            _ => false,
        }
    }

    /// Check if values of these types can be compared with < > <= >= operators.
    pub fn comparable(&self, rhs: &Type, check_quant: bool) -> bool {
        // Only primitive types are comparable
        let primitive_types = [
            std::mem::discriminant(&Type::int(false)),
            std::mem::discriminant(&Type::float(false)),
            std::mem::discriminant(&Type::string(false)),
            std::mem::discriminant(&Type::boolean(false)),
        ];

        let self_discriminant = std::mem::discriminant(self);
        let rhs_discriminant = std::mem::discriminant(rhs);

        if !primitive_types.contains(&self_discriminant)
            || !primitive_types.contains(&rhs_discriminant)
        {
            return false;
        }

        if check_quant && (self.is_optional() || rhs.is_optional()) {
            return false;
        }

        // Int and Float are comparable with each other
        match (self, rhs) {
            (Type::Int { .. }, Type::Float { .. })
            | (Type::Float { .. }, Type::Int { .. })
            | (Type::Int { .. }, Type::Int { .. })
            | (Type::Float { .. }, Type::Float { .. }) => true,
            _ => std::mem::discriminant(self) == std::mem::discriminant(rhs),
        }
    }
}

impl Type {
    /// Resolve a StructInstance type using struct definitions from the document
    pub fn resolve_struct_type(
        &self,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        match self {
            Type::StructInstance {
                type_name,
                members: None,
                optional,
            } => {
                // Find the struct definition
                if let Some(struct_def) = struct_typedefs.iter().find(|s| s.name == *type_name) {
                    Ok(Type::StructInstance {
                        type_name: type_name.clone(),
                        members: Some(struct_def.members.clone()),
                        optional: *optional,
                    })
                } else {
                    Err(WdlError::validation_error(
                        SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                        format!("Unknown struct type: {}", type_name),
                    ))
                }
            }
            // Already resolved or not a struct
            _ => Ok(self.clone()),
        }
    }
}
impl fmt::Display for Type {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let base_str = match self {
            Type::Any { optional: true } => "None".to_string(),
            Type::Any { .. } => "Any".to_string(),
            Type::Boolean { .. } => "Boolean".to_string(),
            Type::Int { .. } => "Int".to_string(),
            Type::Float { .. } => "Float".to_string(),
            Type::String { .. } => "String".to_string(),
            Type::File { .. } => "File".to_string(),
            Type::Directory { .. } => "Directory".to_string(),
            Type::Array {
                item_type,
                nonempty,
                ..
            } => {
                format!("Array[{}]{}", item_type, if *nonempty { "+" } else { "" })
            }
            Type::Map {
                key_type,
                value_type,
                ..
            } => {
                format!("Map[{},{}]", key_type, value_type)
            }
            Type::Pair {
                left_type,
                right_type,
                ..
            } => {
                format!("Pair[{},{}]", left_type, right_type)
            }
            Type::StructInstance { type_name, .. } => type_name.clone(),
            Type::Object { members, .. } => {
                let mut member_strs: Vec<String> = members
                    .iter()
                    .map(|(k, v)| format!("{} : {}", k, v))
                    .collect();
                member_strs.sort();
                format!("object({})", member_strs.join(", "))
            }
        };

        let optional_suffix = if self.is_optional() && !matches!(self, Type::Any { optional: true })
        {
            "?"
        } else {
            ""
        };

        write!(f, "{}{}", base_str, optional_suffix)
    }
}

/// Generate a canonical ID for a struct type based on its members.
fn struct_type_id(members: &HashMap<String, Type>) -> String {
    let mut member_strs: Vec<String> = members
        .iter()
        .map(|(name, ty)| {
            let type_str = if let Type::StructInstance {
                members: Some(nested),
                ..
            } = ty
            {
                format!(
                    "{}{}",
                    struct_type_id(nested),
                    if ty.is_optional() { "?" } else { "" }
                )
            } else {
                ty.to_string()
            };
            format!("{} : {}", name, type_str)
        })
        .collect();
    member_strs.sort();
    format!("struct({})", member_strs.join(", "))
}

/// Unify a list of types into a single type they can all coerce to.
pub fn unify_types(types: Vec<&Type>, check_quant: bool, force_string: bool) -> Type {
    if types.is_empty() {
        return Type::any();
    }

    // Start with first non-String type, or first array type if not checking quantifiers
    let mut unified = if check_quant {
        (*types
            .iter()
            .find(|t| !matches!(t, Type::String { .. } | Type::Any { .. }))
            .unwrap_or(&types[0]))
        .clone()
    } else {
        (*types
            .iter()
            .find(|t| matches!(t, Type::Array { .. }))
            .unwrap_or(
                types
                    .iter()
                    .find(|t| !matches!(t, Type::String { .. } | Type::Any { .. }))
                    .unwrap_or(&types[0]),
            ))
        .clone()
    };

    let mut optional = false;
    let mut all_nonempty = true;
    let mut all_stringifiable = true;

    for ty in &types {
        // Handle optional flag
        if ty.is_optional() {
            optional = true;
        }

        // Handle array nonempty flag
        if !ty.is_nonempty() {
            all_nonempty = false;
        }

        // Check if all types can coerce to String
        if !ty.coerces(&Type::string(true), check_quant) {
            all_stringifiable = false;
        }

        // Promote Int to Float if needed
        if matches!((&unified, ty), (Type::Int { .. }, Type::Float { .. })) {
            unified = Type::float(false);
        }

        // Promote to String in various cases
        if matches!(ty, Type::String { .. })
            && (check_quant || !matches!(&unified, Type::Array { .. }))
            && !matches!(&unified, Type::Pair { .. } | Type::Map { .. })
        {
            unified = Type::string(false);
        }
    }

    // Apply optional and nonempty flags
    if matches!(&unified, Type::Array { .. }) {
        let backup = unified.clone();
        unified = unified.with_nonempty(all_nonempty).unwrap_or(backup);
    }
    unified = unified.with_optional(optional);

    // Check if all types can coerce to our unified type
    for ty in &types {
        if !ty.coerces(&unified, check_quant) {
            if all_stringifiable && force_string {
                return Type::string(optional);
            }
            return Type::any();
        }
    }

    unified
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_type_creation() {
        let int_type = Type::int(false);
        assert!(!int_type.is_optional());
        assert_eq!(int_type.to_string(), "Int");

        let opt_int = Type::int(true);
        assert!(opt_int.is_optional());
        assert_eq!(opt_int.to_string(), "Int?");
    }

    #[test]
    fn test_array_type() {
        let arr = Type::array(Type::int(false), false, true);
        assert!(arr.is_nonempty());
        assert_eq!(arr.to_string(), "Array[Int]+");

        let opt_arr = Type::array(Type::string(false), true, false);
        assert!(opt_arr.is_optional());
        assert!(!opt_arr.is_nonempty());
        assert_eq!(opt_arr.to_string(), "Array[String]?");
    }

    #[test]
    fn test_map_type() {
        let map = Type::map(Type::string(false), Type::int(false), false);
        assert_eq!(map.to_string(), "Map[String,Int]");

        let opt_map = Type::map(Type::string(false), Type::float(false), true);
        assert_eq!(opt_map.to_string(), "Map[String,Float]?");
    }

    #[test]
    fn test_pair_type() {
        let pair = Type::pair(Type::int(false), Type::string(false), false);
        assert_eq!(pair.to_string(), "Pair[Int,String]");
    }

    #[test]
    fn test_basic_coercion() {
        let int_type = Type::int(false);
        let float_type = Type::float(false);
        let string_type = Type::string(false);

        // Int coerces to Float
        assert!(int_type.coerces(&float_type, true));

        // Int coerces to String
        assert!(int_type.coerces(&string_type, true));

        // Float doesn't coerce to Int
        assert!(!float_type.coerces(&int_type, true));

        // String coerces to Int
        assert!(string_type.coerces(&int_type, true));
    }

    #[test]
    fn test_optional_coercion() {
        let int_type = Type::int(false);
        let opt_int = Type::int(true);
        let opt_float = Type::float(true);

        // Non-optional coerces to optional
        assert!(int_type.coerces(&opt_int, true));
        assert!(int_type.coerces(&opt_float, true));

        // Optional doesn't coerce to non-optional (with quantifier checking)
        assert!(!opt_int.coerces(&int_type, true));

        // But does coerce without quantifier checking
        assert!(opt_int.coerces(&int_type, false));
    }

    #[test]
    fn test_array_coercion() {
        let int_arr = Type::array(Type::int(false), false, false);
        let float_arr = Type::array(Type::float(false), false, false);
        let string_type = Type::string(false);

        // Array[Int] coerces to Array[Float]
        assert!(int_arr.coerces(&float_arr, true));

        // Array[Int] coerces to String
        assert!(int_arr.coerces(&string_type, true));
    }

    #[test]
    fn test_equatable_types() {
        let int_type = Type::int(false);
        let float_type = Type::float(false);
        let string_type = Type::string(false);
        let file_type = Type::file(false);

        // Int and Float are equatable at top level
        assert!(int_type.equatable(&float_type, false));

        // Also equatable in compound types (arrays, maps, etc.)
        assert!(int_type.equatable(&float_type, true));

        // Same types are always equatable
        assert!(string_type.equatable(&string_type, true));

        // File and String should be equatable for placeholder coercion
        // per WDL spec: "File is substituted as if it were a String"
        assert!(file_type.equatable(&string_type, false));
        assert!(string_type.equatable(&file_type, false));
        assert!(file_type.equatable(&string_type, true));
        assert!(string_type.equatable(&file_type, true));
    }

    #[test]
    fn test_comparable_types() {
        let int_type = Type::int(false);
        let float_type = Type::float(false);
        let string_type = Type::string(false);
        let opt_int = Type::int(true);

        // Int and Float are comparable
        assert!(int_type.comparable(&float_type, true));

        // String is comparable with String
        assert!(string_type.comparable(&string_type, true));

        // Optional types aren't comparable when checking quantifiers
        assert!(!opt_int.comparable(&int_type, true));

        // But are comparable when not checking quantifiers
        assert!(opt_int.comparable(&int_type, false));
    }

    #[test]
    fn test_type_parameters() {
        let arr = Type::array(Type::int(false), false, false);
        let params = arr.parameters();
        assert_eq!(params.len(), 1);
        assert!(matches!(params[0], Type::Int { .. }));

        let map = Type::map(Type::string(false), Type::float(false), false);
        let map_params = map.parameters();
        assert_eq!(map_params.len(), 2);
    }

    #[test]
    fn test_with_optional() {
        let int_type = Type::int(false);
        let opt_int = int_type.clone().with_optional(true);

        assert!(!int_type.is_optional());
        assert!(opt_int.is_optional());
        assert_eq!(opt_int.to_string(), "Int?");
    }

    #[test]
    fn test_unify_types() {
        let int_type = Type::int(false);
        let float_type = Type::float(false);
        let types = vec![&int_type, &float_type];
        let unified = unify_types(types, true, false);
        assert!(matches!(unified, Type::Float { .. }));

        let bool_type = Type::boolean(false);
        let string_types = vec![&int_type, &bool_type];
        let unified_string = unify_types(string_types, true, true);
        assert!(matches!(unified_string, Type::String { .. }));
    }

    #[test]
    fn test_any_type() {
        let any_type = Type::any();
        let none_type = Type::none();
        let int_type = Type::int(false);

        // Any coerces to everything
        assert!(any_type.coerces(&int_type, true));
        assert!(int_type.coerces(&any_type, true));

        // None (optional Any) has different string representation
        assert_eq!(any_type.to_string(), "Any");
        assert_eq!(none_type.to_string(), "None");
    }

    #[test]
    fn test_struct_type_id() {
        let mut members1 = HashMap::new();
        members1.insert("a".to_string(), Type::int(false));
        members1.insert("b".to_string(), Type::string(false));

        let mut members2 = HashMap::new();
        members2.insert("b".to_string(), Type::string(false));
        members2.insert("a".to_string(), Type::int(false));

        // Same members should produce same ID regardless of order
        assert_eq!(struct_type_id(&members1), struct_type_id(&members2));
    }
}
