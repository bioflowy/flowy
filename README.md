# Flowy - miniwdl Go Port

A Go port of [miniwdl](https://github.com/chanzuckerberg/miniwdl), a WDL (Workflow Description Language) runtime and static analyzer.

## Project Structure

```
pkg/
├── errors/    # WDL error types and handling
├── utils/     # Utility functions
├── env/       # Environment and bindings
├── types/     # WDL type system
├── values/    # WDL value system
└── expr/      # WDL expression AST
```

## Development

This project is developed by porting miniwdl components incrementally, starting from the least dependent modules:

1. Error handling and types
2. Utility functions  
3. Environment and bindings
4. Type system
5. Value system
6. Expression system

Each package is fully tested before proceeding to the next phase.