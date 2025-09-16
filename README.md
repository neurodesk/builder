# Builder

A tool to build container images from recipes using templates.

## Configuration

The builder uses a YAML configuration file (default: `builder.config.yaml`) to specify various settings:

```yaml
# Recipe directories to search for build recipes
recipe_roots:
  - /path/to/recipes

# Include directories for shared build components
include_dirs:
  - /path/to/includes

# Optional: Template directory to override built-in templates
template_dir: /path/to/custom/templates
```

## Template Directory Override

The `template_dir` configuration option allows you to provide custom templates that override the built-in templates. This is useful for:

- Customizing existing templates for your specific needs
- Testing template modifications before contributing back
- Organization-specific template variations

### How It Works

1. When a template is requested, the builder first checks the `template_dir` for a file named `{template_name}.yaml`
2. If found, it loads and validates the custom template
3. If not found or invalid, it falls back to the built-in template
4. If neither exists, an error is returned

### Example

To override the built-in `jq` template:

1. Create a custom template file: `/path/to/custom/templates/jq.yaml`
2. Configure your `builder.config.yaml`:
   ```yaml
   recipe_roots:
     - /path/to/recipes
   include_dirs:
     - /path/to/includes
   template_dir: /path/to/custom/templates
   ```
3. Use the template in your recipes as normal - the custom version will be used automatically

### Template Format

Custom templates must follow the same format as built-in templates:

```yaml
name: template-name
url: https://example.com
binaries:
  arguments:
    required:
    - version
  dependencies:
    apt:
    - package1
    - package2
  instructions: |
    {{ self.install_dependencies() }}
    # Custom installation commands here
source:
  # Alternative installation method
  instructions: |
    # Source installation commands here
```

## Usage

```bash
# Generate Dockerfile for a recipe
builder --config builder.config.yaml generate recipe-name

# Test all recipes
builder --config builder.config.yaml test-all
```

## Development

### Building

```bash
go build ./cmd/builder
```

### Testing

```bash
go test ./...
```