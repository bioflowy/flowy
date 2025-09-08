version 1.2

struct Person {
    String name
    Int age
    String? email
}

workflow test_struct {
    input {
        String person_name
        Int person_age
    }
    
    # Test struct literal creation
    Person person = {
        "name": person_name,
        "age": person_age
    }
    
    # Test member access
    String name_result = person.name
    Int age_result = person.age
    
    output {
        String out_name = name_result
        Int out_age = age_result
        Person out_person = person
    }
}