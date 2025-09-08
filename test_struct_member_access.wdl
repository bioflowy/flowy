version 1.2

struct Person {
    String name
    Int age
    String? email
}

struct Company {
    String name
    Person founder
}

workflow test_nested_struct_access {
    input {
        String company_name
        String founder_name
        Int founder_age
    }
    
    # Test nested struct creation
    Person founder = {
        "name": founder_name,
        "age": founder_age,
        "email": "founder@company.com"
    }
    
    Company company = {
        "name": company_name,
        "founder": founder
    }
    
    # Test nested member access
    Person founder_result = company.founder
    
    output {
    }
}