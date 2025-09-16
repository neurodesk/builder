# Starlark Scripting Example

This example demonstrates how Starlark scripts can be used in place of traditional neurodocker templates for dynamic package installation and configuration.

## Example: Dynamic FSL Installation

Here's how you could replace a neurodocker template with a Starlark script:

```yaml
# build.yaml
name: fsl-neurodesk
version: "6.0.5"
architectures:
  - x86_64

build:
  kind: neurodocker
  base-image: "ubuntu:20.04"
  pkg-manager: apt
  
  directives:
    - starlark:
        script: |
          # Access parameters and context
          version = context.version
          pkg_manager = context.PackageManager
          
          # Dynamic package selection based on version
          def install_fsl():
              # Base packages always needed
              base_packages = ["curl", "wget", "python3", "python3-pip"]
              
              # Install base packages
              for pkg in base_packages:
                  install_packages(pkg)
              
              # Version-specific logic
              if version.startswith("6.0"):
                  # FSL 6.0.x specific packages
                  install_packages("libopenblas-base", "libgomp1")
                  
                  # Set environment for FSL 6.0.x
                  set_environment("FSLDIR", "/usr/local/fsl")
                  set_environment("FSLVERSION", version)
                  set_environment("PATH", "/usr/local/fsl/bin:$PATH")
                  
                  # Download and install FSL
                  download_url = "https://fsl.fmrib.ox.ac.uk/fsldownloads/fsl-" + version + "-centos7_64.tar.gz"
                  
                  run_command("mkdir -p /usr/local")
                  run_command("curl -fsSL " + download_url + " | tar -xz -C /usr/local")
                  run_command("mv /usr/local/fsl-" + version + " /usr/local/fsl")
                  
              elif version.startswith("5."):
                  # FSL 5.x specific installation
                  install_packages("fsl-core", "fsl-atlases")
                  set_environment("FSLDIR", "/usr/share/fsl/5.0")
              
              # Common post-installation setup
              run_command("chmod +x /usr/local/fsl/bin/*")
              
              # Set computed variables for later use
              set_variable("fsl_installed", True)
              set_variable("fsl_bindir", "/usr/local/fsl/bin")
          
          # Execute the installation
          install_fsl()

    # Additional directives can use Starlark variables
    - environment:
        FSL_INSTALLED: "{{ fsl_installed }}"
        
    - test:
        name: "fsl_version_check"
        script: "{{ fsl_bindir }}/fslinfo"
```

## Example: Conditional Installation Based on Architecture

```yaml
directives:
  - starlark:
      script: |
        # Get system information from context
        arch = "x86_64"  # This would come from context in real usage
        enable_gpu = False  # This could be a parameter
        
        def install_dependencies():
            # Architecture-specific packages
            if arch == "x86_64":
                install_packages("libc6-dev", "gcc")
                if enable_gpu:
                    install_packages("nvidia-cuda-toolkit")
            elif arch == "aarch64":
                install_packages("libc6-dev-arm64-cross", "gcc-aarch64-linux-gnu")
                # No GPU support for ARM in this example
            
            # Common packages regardless of arch
            install_packages("build-essential", "cmake", "git")
            
            # Set arch-specific environment
            set_environment("TARGET_ARCH", arch)
            set_environment("GPU_ENABLED", str(enable_gpu).lower())
        
        install_dependencies()
```

## Example: Complex Multi-step Installation

```yaml
directives:
  - variables:
      matlab_version: "R2023a"
      install_toolboxes: true
      
  - starlark:
      script: |
        matlab_version = context.matlab_version
        install_toolboxes = context.install_toolboxes
        
        def setup_matlab():
            # Phase 1: Install system dependencies
            system_deps = [
                "libxt6", "libxmu6", "libxpm4", "libxaw7", 
                "libasound2", "libxtst6", "libxi6"
            ]
            
            for dep in system_deps:
                install_packages(dep)
            
            # Phase 2: Download MATLAB Runtime
            if matlab_version == "R2023a":
                mcr_url = "https://ssd.mathworks.com/supportfiles/downloads/R2023a/Release/6/deployment_files/installer/complete/glnxa64/MATLAB_Runtime_R2023a_Update_6_glnxa64.zip"
            elif matlab_version == "R2022b":
                mcr_url = "https://ssd.mathworks.com/supportfiles/downloads/R2022b/Release/9/deployment_files/installer/complete/glnxa64/MATLAB_Runtime_R2022b_Update_9_glnxa64.zip"
            else:
                # Fallback to latest
                mcr_url = "https://ssd.mathworks.com/supportfiles/downloads/R2023a/Release/6/deployment_files/installer/complete/glnxa64/MATLAB_Runtime_R2023a_Update_6_glnxa64.zip"
            
            # Download and install
            run_command("mkdir -p /tmp/matlab")
            run_command("cd /tmp/matlab && curl -fsSL " + mcr_url + " -o mcr.zip")
            run_command("cd /tmp/matlab && unzip -q mcr.zip")
            run_command("cd /tmp/matlab && ./install -mode silent -agreeToLicense yes -destinationFolder /opt/mcr")
            
            # Phase 3: Set up environment
            mcr_version = matlab_version.replace("R", "v").replace("a", ".1").replace("b", ".2")
            mcr_root = "/opt/mcr/" + mcr_version
            
            set_environment("MCR_ROOT", mcr_root)
            set_environment("LD_LIBRARY_PATH", mcr_root + "/runtime/glnxa64:" + mcr_root + "/bin/glnxa64:" + mcr_root + "/sys/os/glnxa64:" + mcr_root + "/extern/bin/glnxa64")
            set_environment("MATLAB_VERSION", matlab_version)
            
            # Phase 4: Optional toolboxes
            if install_toolboxes:
                # Install additional MATLAB toolboxes or compiled applications
                run_command("echo 'Installing additional MATLAB toolboxes...'")
                # Toolbox-specific installation commands would go here
            
            # Clean up
            run_command("rm -rf /tmp/matlab")
            
            # Set completion marker
            set_variable("matlab_installed", True)
            set_variable("mcr_path", mcr_root)
        
        setup_matlab()
```

## Benefits of Starlark over Traditional Templates

1. **Full Programming Language**: Starlark provides conditionals, loops, functions, and complex data structures
2. **Dynamic Logic**: Make installation decisions based on runtime conditions
3. **Parameter Access**: Full access to all build parameters and context variables
4. **Modular**: Functions can be reused and composed
5. **Error Handling**: Better error messages and debugging capabilities
6. **Integration**: Seamlessly integrates with existing Jinja2 templates and directives

## Available Built-in Functions

- `install_packages(pkg1, pkg2, ...)` - Install system packages
- `set_variable(name, value)` - Set variables for use in other directives
- `run_command(command)` - Execute shell commands
- `set_environment(key, value)` - Set environment variables
- `print(...)` - Debug output

## Context Variables Available

All template variables are available as attributes of the `context` and `local` objects in Starlark scripts:
- `context.version` - Package version
- `context.PackageManager` - Package manager being used
- User-defined variables from the `variables` directive (e.g., `context.my_variable`)
- Variables set by previous directives

The `context` and `local` objects provide the same variables - they are aliases for convenience.