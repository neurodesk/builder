# Neurodesk Builder

A container build system with support for dynamic Jinja2 templating and Starlark scripting.

## Features

- **Template System**: Jinja2-compatible templating for dynamic build configuration
- **Starlark Scripting**: Full programming language support for complex build logic
- **Modular Directives**: Composable build instructions with validation
- **Recipe System**: YAML-based build recipes with parameter support
- **Neurodocker Compatibility**: Designed to replace and extend neurodocker templates

## New: Starlark Scripting Support

This builder now supports Starlark scripts for dynamic container builds. Starlark provides a Python-like programming language that integrates seamlessly with the existing Jinja2 template system.

### Key Benefits

- **Dynamic Logic**: Make build decisions based on runtime conditions
- **Full Programming Language**: Conditionals, loops, functions, and data structures
- **Parameter Access**: Complete access to build parameters and context variables
- **Modular Functions**: Reusable and composable build logic
- **Better Error Handling**: Clear error messages and debugging capabilities

### Quick Example

```yaml
# build.yaml
name: my-app
version: "1.0.0"

build:
  kind: neurodocker
  base-image: "ubuntu:20.04"
  pkg-manager: apt
  
  directives:
    - variables:
        enable_gpu: true
        python_version: "3.9"
    
    - starlark:
        script: |
          # Access template variables
          version = "{{ version }}"
          enable_gpu = {{ enable_gpu }}
          py_version = "{{ python_version }}"
          
          def setup_environment():
              # Install base packages
              install_packages("curl", "wget", "git")
              
              # Python installation
              if py_version == "3.9":
                  install_packages("python3.9", "python3.9-dev")
              else:
                  install_packages("python3", "python3-dev")
              
              # GPU support
              if enable_gpu:
                  install_packages("nvidia-cuda-toolkit")
                  set_environment("CUDA_ENABLED", "true")
              
              # Set up application
              set_environment("APP_VERSION", version)
              run_command("python3 --version")
          
          setup_environment()
```

### Available Functions

Starlark scripts have access to these built-in functions:

- `install_packages(pkg1, pkg2, ...)` - Install system packages
- `set_variable(name, value)` - Set variables for use in other directives  
- `run_command(command)` - Execute shell commands
- `set_environment(key, value)` - Set environment variables
- `print(...)` - Debug output

### Context Variables

All Jinja2 template variables are available in Starlark scripts:
- `version` - Package version
- `context` - Full recipe context
- User-defined variables from the `variables` directive
- Variables set by previous directives

## Getting Started

1. Create a `build.yaml` file with your build configuration
2. Use traditional directives or Starlark scripts for complex logic
3. Build your container using the builder CLI

See the [examples/](examples/) directory for more comprehensive examples.

## Examples

- [Starlark Usage Guide](examples/starlark_usage.md) - Comprehensive examples and best practices
- [Simple Demo](examples/starlark-demo.yaml) - Basic Starlark functionality demonstration

## Architecture

- `pkg/jinja2/` - Jinja2-compatible template engine
- `pkg/starlark/` - Starlark scripting support and value conversion
- `pkg/recipe/` - Build recipe system with directive validation
- `pkg/ir/` - Intermediate representation for build instructions
- `pkg/templates/` - Traditional neurodocker-style templates

## Migration from Neurodocker

The Starlark scripting system is designed to replace traditional neurodocker templates while providing much more flexibility and power. See the [Starlark Usage Guide](examples/starlark_usage.md) for migration examples.

## Contributing

Contributions are welcome! Please ensure that:

1. All tests pass (`go test ./...`)
2. New functionality includes comprehensive tests
3. Code follows the existing patterns and conventions
4. Documentation is updated for user-facing changes

## License

[License information would go here]