# ARM64 Template Audit

Last updated: 2026-03-26

This document tracks:

- current arm64 status of built-in macro-backed templates in `pkg/recipe/template_macros`
- which Neurocontainers recipes use those templates
- which recipes use non-Ubuntu base images, so we can prioritize arm64 image-pull checks
- builder-side fixes already landed to improve arm64 support

The recipe inventory below was scanned from:

- `/home/joshua/dev/projects/neurocontainers/recipes`

## Current Template Status

Status meanings:

- `Works`: confirmed by building a minimal `aarch64` recipe and running a smoke check
- `Fails`: confirmed by build failure or runtime architecture mismatch on arm64
- `Unknown`: not yet closed out with a clean final result

| Template | Method | Arm64 Status | Notes |
|---|---|---:|---|
| `vnc` | `source` | Works | Builds and VNC password file generation works |
| `dcm2niix` | `source` | Works | Builds from source and runs on arm64 |
| `miniconda` | `binaries` | Works | Macro-backed template now selects the correct arm64 installer URL and a clean minimal `aarch64` smoke build completes with `conda --version` |
| `jq` | `binaries` | Fails | Downloads `jq-linux64`; runtime `Exec format error` |
| `bids_validator` | `binaries` | Fails | Macro regression fixed; arm64 now reaches `npm install`, but native addon build still fails (`make` missing on Ubuntu 22.04 template test, `distutils` missing on Ubuntu 24.04 via `bidscoin`) |
| `neurodebian` | `binaries` | Fails | Broken key import: `gpg: no valid OpenPGP data found` |
| `ndfreeze` | `source` | Fails | Build stalls/fails during `nd_freeze 2024-01-01` apt refresh |
| `dcm2niix` | `binaries` | Fails | Binary payload is wrong architecture; runtime `Exec format error` |
| `convert3d` | `binaries` | Fails | Downloads x86_64 tarball; runtime `Exec format error` |
| `matlabmcr` | `binaries` | Fails by design | Upstream is x86_64-only |
| `ants` | `source` | Unknown | Still needs a clean final arm64 run |
| `mrtrix3` | `source` | Unknown | Still needs a clean final arm64 run |
| `niftyreg` | `source` | Works | `./build.sh niftyreg` completes on arm64 and `reg_aladin -h` runs in the built image |
| `fsl` | `binaries` | Unknown | Still needs a clean final arm64 run |
| `afni` | `binaries` | Likely fails | Template contains x86_64-specific payloads/debs |
| `afni` | `source` | Likely fails | Template contains x86_64-specific library paths and symlinks |
| `ants` | `binaries` | Likely fails | Template points at X64 release artifacts |
| `cat12` | `binaries` | Likely fails | MATLAB runtime based |
| `freesurfer` | `binaries` | Likely fails | Release artifacts are x86_64 |
| `minc` | `binaries` | Likely fails | Uses x86_64 Debian package artifact |
| `mricron` | `binaries` | Likely fails | Legacy binary release likely x86_64-only |
| `mrtrix3` | `binaries` | Likely fails | Release artifact appears x86_64-oriented |
| `petpvc` | `binaries` | Likely fails | Legacy binary tarball likely x86_64-only |
| `spm12` | `binaries` | Likely fails | MATLAB runtime based |

## Highest-Priority Template Work

Priority is based on confirmed failure plus recipe usage count.

| Template | Method | Unique Recipes Using It |
|---|---|---:|
| `miniconda` | `binaries` | 39 |
| `matlabmcr` | `binaries` | 16 |
| `fsl` | `binaries` | 8 |
| `freesurfer` | `binaries` | 7 |
| `ants` | `binaries` + `source` | 7 total |
| `mrtrix3` | `binaries` + `source` | 6 total |
| `dcm2niix` | `binaries` + `source` | 5 total |
| `convert3d` | `binaries` | 2 |
| `spm12` | `binaries` | 2 |
| `bids_validator` | `binaries` | 1 |
| `afni` | `binaries` | 1 |
| `minc` | `binaries` | 1 |

Recommended fix order:

1. `fsl/binaries`
2. `matlabmcr/binaries`
3. `freesurfer/binaries`
4. `ants/binaries`
5. `dcm2niix/binaries`
6. `mrtrix3/binaries`
7. `convert3d/binaries`

## Builder Changes Landed

These changes are now in this repo and should be used as the new baseline for arm64 work.

### Local wrapper compatibility: `build.sh`

- On 2026-03-26, `./build.sh bidscoin` on an `aarch64` host failed immediately before any recipe build work with:
  `unknown flag: --ignore-architectures`
- Cause: the local wrapper script had been changed to pass `build --ignore-architectures`, but the current `cmd/builder` CLI does not define that flag.
- Fix landed: restore `build.sh` to call `builder build "$RECIPE"` without the unsupported option.
- Result after the fix: the same `./build.sh bidscoin` invocation progressed into the real Docker build on arm64 and reached Dockerfile step `7/19` (`apt-get ... install`) instead of failing in argument parsing.
- Scope note: this closes a local wrapper regression, not an arm64 template or recipe readiness check for `bidscoin`.

### Builder architecture selection: multi-arch recipe rendering

- On 2026-03-26, `./build.sh bidscoin` was rerun on an `aarch64` host after the wrapper fix.
- The generated Dockerfile still rendered the `miniconda` installer as:
  `https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-x86_64.sh`
  even though the host Docker engine was `arm64` and the recipe declares both:
  `architectures: [x86_64, aarch64]`
- Cause: `pkg/recipe/recipe.go` defaulted the template/render context architecture to the first declared recipe architecture instead of preferring the current host architecture when it was supported by the recipe.
- Fix landed:
  - map the current host `GOARCH` into builder architecture names (`amd64 -> x86_64`, `arm64 -> aarch64`)
  - prefer that host architecture during build generation when the recipe explicitly declares it
  - add a regression test covering a dual-architecture recipe that lists the non-host architecture first
- Verified result after the fix:
  - `go test ./pkg/recipe/...` passes
  - regenerating `./build.sh bidscoin` on the same `aarch64` host now emits:
    `https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-aarch64.sh`
    in `local/build/bidscoin/Dockerfile`
- Scope note: this closes one builder-side architecture-selection regression. The `bidscoin` build still has a later arm64 blocker in the shared `bids_validator` / Node install path, so this is not a full recipe close-out.

### Recipe-level build check: `xnat`

- On 2026-03-26, `./build.sh xnat` on an `aarch64` host initially failed before the Docker build started with:
  `detected unrendered string concatenation in generated Dockerfile; fix recipe/templates`
- Cause: `neurocontainers/recipes/xnat/build.yaml` used `get_file("xnat-web-" + context.version + ".war")`, which the current builder intentionally rejects.
- Fix landed: stage the WAR as a stable local filename (`xnat-web.war`) and reference it with `get_file("xnat-web.war")`.
- Result after the fix: the same build progressed into the real Docker build and package-install steps on arm64 instead of failing during recipe/render processing.
- Scope note: `xnat` still declares `architectures: [x86_64]`, so this was a builder compatibility fix, not evidence that the recipe is arm64-ready.

### Recipe-level build check: `mipav`

- On 2026-03-26, `./build.sh mipav` was run on an `aarch64` host.
- Initial failure:
  `sh: 0: cannot open /.neurocontainer-cache/mipav_unix_+ context.version +.sh: No such file`
- Cause: `neurocontainers/recipes/mipav/build.yaml` used `get_file("mipav_unix_"+ context.version +".sh")`, which the current builder passed through literally into the generated Dockerfile.
- Fix landed: stage the installer as a stable local filename (`mipav_unix.sh`) and reference it with `get_file("mipav_unix.sh")`.
- Verified rerun result:
  - the same `./build.sh mipav` invocation now gets past the recipe/render issue and starts the bundled installer on arm64
  - the build then fails later with:
    `/.neurocontainer-cache/mipav_unix.sh: ... /tmp/mipav_unix.sh.10.dir/jre/bin/java: Exec format error`
- Current remaining blocker after this fix:
  - the upstream MIPAV installer bundles an x86_64 JRE, so the install step is not arm64-compatible
- Scope note: `mipav` still declares `architectures: [x86_64]`; this closes one recipe-side builder compatibility issue but does not make the recipe arm64-ready.

### Recipe-level build check: `niimath`

- On 2026-03-26, `./build.sh niimath` was run on an `aarch64` host.
- Result:
  - the Docker build completed successfully and produced `niimath:1.0.20250804`
  - a follow-up runtime smoke check with `docker run --rm niimath:1.0.20250804 /usr/bin/niimath` returned the expected usage text
  - `docker run --rm niimath:1.0.20250804 file /usr/bin/niimath` reported:
    `ELF 64-bit LSB executable, ARM aarch64`
- Scope note: this is a successful recipe-level arm64 build and smoke-check result for `niimath`; no recipe changes were required.

### Recipe-level build check: `builder`

- On 2026-03-26, `./build.sh builder` was run on an `aarch64` host.
- Result:
  - the Docker build completed successfully and produced `builder:0.2`
  - `docker image inspect builder:0.2 --format '{{.Architecture}} {{.Os}}'` reported:
    `arm64 linux`
- Scope note: this is a successful recipe-level arm64 build result for `builder`; no recipe changes were required.

### Recipe-level build check: `niftyreg`

- On 2026-03-26, `./build.sh niftyreg` was run on an `aarch64` host.
- Result:
  - the Docker build completed successfully and produced `niftyreg:1.4.0`
  - a follow-up runtime smoke check with `docker run --rm niftyreg:1.4.0 bash -lc 'command -v reg_aladin && reg_aladin -h | head -n 5'` confirmed the binary is present and starts normally
  - `docker image inspect niftyreg:1.4.0 --format '{{.Architecture}} {{.Os}}'` reported:
    `arm64 linux`
- Scope note: this is a successful recipe-level arm64 build and smoke-check result for `niftyreg`; no recipe changes were required.

### Recipe-level build check: `apptainer`

- On 2026-03-26, `./build.sh apptainer` was run on an `aarch64` host.
- Initial failure:
  `apptainer:amd64` could not be installed in the arm64 image because the recipe staged the upstream amd64 `.deb` and `apt` then reported unmet `:amd64` dependencies.
- Cause:
  - `neurocontainers/recipes/apptainer/build.yaml` only declared `architectures: [x86_64]`
  - the build path only supported the upstream amd64 Debian package
- Fix landed in recipe YAML:
  - add `aarch64` as a declared recipe architecture
  - keep the existing amd64 `.deb` install path for `x86_64`
  - add an arm64 source-build path using the upstream release tarball, arm64 Go toolchain, and required build dependencies
  - remove top-level arch-specific URL variables that failed IR rendering on arm64
  - run `./mconfig --without-suid` so source configuration works in this container environment
- Verified rerun result:
  - the same `./build.sh apptainer` invocation now gets past the wrong-architecture package failure, completes source configuration, and runs through the upstream arm64 compile/install path
  - the rerun reached the recipe validation step `apptainer --version` successfully and then entered Docker layer export
- Current status after this fix:
  - no new hard arm64 build failure was hit after the packaging fix
  - I interrupted the build during Docker's final image export, and `docker image inspect apptainer:1.4.4` confirms no local image was finalized from that interrupted run
- Scope note: this closes one concrete recipe-side arm64 build issue for `apptainer` by replacing the invalid amd64 package path with an arm64-capable source build path.

### Recipe-level build check: `brainlifecli`

- On 2026-03-26, `./build.sh brainlifecli` was run on an `aarch64` host.
- Result:
  - the Docker build completed successfully and produced `brainlifecli:1.7.0`
  - `docker image inspect brainlifecli:1.7.0 --format '{{.Architecture}} {{.Os}}'` reported:
    `arm64 linux`
  - a follow-up runtime smoke check with `docker run --rm brainlifecli:1.7.0 sh -lc 'command -v bl && bl --help | head -n 5'` confirmed the `bl` entrypoint is present at `/usr/bin/bl` and starts normally
- Scope note: this is a successful recipe-level arm64 build and smoke-check result for `brainlifecli`; no recipe changes were required.

### Recipe-level build check: `bidstools`

- On 2026-03-26, `./build.sh bidstools` was run on an `aarch64` host.
- Initial failure:
  `conda: not found`
- Cause:
  - `neurocontainers/recipes/bidstools/build.yaml` only declared `architectures: [x86_64]`
  - because of that, the `miniconda` template rendered the x86_64 installer on an arm64 host, and the later `conda update` step failed because a usable arm64 `conda` was never installed
- Fix landed in recipe YAML:
  - add `aarch64` as a declared recipe architecture in `neurocontainers/recipes/bidstools/build.yaml`
- Verified rerun result:
  - the regenerated Dockerfile now stages `https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-aarch64.sh` instead of the x86_64 installer
  - rerunning `./build.sh bidstools` no longer failed at `conda: not found`; it progressed into the Miniconda bootstrap and reached:
    `PREFIX=/opt/miniconda-latest`
    `Installing base environment...`
- Current status after this fix:
  - I interrupted the long rerun during the Miniconda/base-environment setup, so there is not yet a finalized `bidstools:1.0.4` image from the rerun
- Scope note: this closes one concrete arm64 build issue for `bidstools` by making the recipe render the correct Miniconda installer for arm64 hosts.

### Recipe-level build check: `gingerale`

- On 2026-03-26, `./build.sh gingerale` was run on an `aarch64` host.
- Initial failure:
  `fetching "https://www.brainmap.org/ale/GingerALE.jar": ... x509: certificate signed by unknown authority`
- Cause:
  - `neurocontainers/recipes/gingerale/build.yaml` pointed at an HTTPS endpoint whose certificate chain is not trusted in this builder environment
- Fix landed in recipe YAML:
  - switch the staged JAR URL from `https://www.brainmap.org/ale/GingerALE.jar` to the working `http://www.brainmap.org/ale/GingerALE.jar` endpoint
- Verified rerun result:
  - the Docker build completed successfully and produced `gingerale:3.0.2`
  - `docker image inspect gingerale:3.0.2 --format '{{.Architecture}} {{.Os}}'` reported:
    `arm64 linux`
  - a follow-up runtime probe confirmed the launcher script exists at `/opt/gingerale/gingerale`
  - invoking the JAR directly now reaches Java startup and fails with the expected headless AWT message rather than a build-time fetch error:
    `java.awt.HeadlessException`
    `No X11 DISPLAY variable was set`
- Scope note: this closes one recipe-side build/fetch issue for `gingerale`; the built image is arm64, and the remaining runtime limitation from the smoke check is GUI/headless related rather than an arm64 packaging failure.

### Recipe-level build check: `blastct`

- On 2026-03-26, `./build.sh blastct` was run on an `aarch64` host.
- Initial failure:
  `conda: not found`
- Cause:
  - `neurocontainers/recipes/blastct/build.yaml` only declared `architectures: [x86_64]`
  - because of that, the `miniconda` template rendered the x86_64 installer on an arm64 host, and the later `conda config` step failed because a usable arm64 `conda` was never installed
- Fix landed in recipe YAML:
  - add `aarch64` as a declared recipe architecture in `neurocontainers/recipes/blastct/build.yaml`
- Verified rerun result:
  - the regenerated Dockerfile now stages `https://repo.anaconda.com/miniconda/Miniconda3-py311_25.5.1-0-Linux-aarch64.sh` instead of the x86_64 installer
  - rerunning `./build.sh blastct` no longer failed at `conda: not found`; it progressed through Miniconda bootstrap and past the first `conda config` step
  - the rerun then failed later at:
    `CondaValueError: 'base' is a reserved environment name`
    from `conda create -y -q --name base`
- Current status after this fix:
  - the original arm64 Miniconda/render issue is fixed
  - `blastct` still has a separate recipe problem to resolve before the image can complete on arm64, because the recipe asks modern Conda to create a new environment named `base`
- Scope note: this closes one concrete arm64 build/render issue for `blastct` by making the recipe render the correct Miniconda installer for arm64 hosts; the remaining failure is a separate Conda-environment logic issue.

### Recipe-level full test check: `xnat`

- On 2026-03-26, `./test.sh xnat` was run against the existing local `xnat:1.9.2.1` image without rebuilding it.
- Initial result: `94/98` tests passed. The four failures were in `neurocontainers/recipes/xnat/fulltest.yaml`, not the container itself:
  `Build YAML exists`, `Build YAML version`, `Build YAML name`, and `README exists`.
- Cause: the full test incorrectly assumed source-tree artifacts (`/build.yaml`, `/README.md`) would exist inside the runtime container image.
- Fix landed: replace those checks with assertions against shipped XNAT artifacts that actually exist in the container:
  `/opt/xnat-webapp/META-INF/MANIFEST.MF`,
  `/opt/xnat-webapp/resources/samples/xnat-conf.properties`,
  and `/opt/xnat-webapp/scripts/generated/xnat_projectData.js`.
- Verified rerun result: the same `./test.sh xnat` invocation then passed cleanly with `98/98` tests passing in `27.7s`.
- Scope note: this closes a recipe YAML/fulltest mismatch for `xnat`; it does not change the recipe's declared architecture support.

### Recipe-level full test check: `sigviewer`

- On 2026-03-26, `./test.sh sigviewer` was run against the existing local `sigviewer:0.6.4` image on an `aarch64` host without rebuilding the Docker image.
- Initial result: `29/34` tests passed. All five failures were in `neurocontainers/recipes/sigviewer/fulltest.yaml`, not the container runtime itself:
  `Libbiosig present`, `README available`, `Module load message`, `Example usage documented`, and `Documentation link present`.
- Cause:
  - the full test hardcoded `/usr/lib/x86_64-linux-gnu/libbiosig.so*`, which is wrong on arm64 where the package installs under `/usr/lib/aarch64-linux-gnu/`
  - the full test also assumed `/README.md` would be present in the runtime image, but this image only ships the Debian-packaged docs at `/usr/share/doc/sigviewer/README.md.gz`
- Fix landed in recipe YAML only: make the libbiosig path architecture-agnostic and replace the `/README.md` assertions with checks against the packaged `README.md.gz`.
- Verified rerun result: the same `./test.sh sigviewer` invocation then passed cleanly with `34/34` tests passing in `42.4s`.
- Scope note: this closes a recipe YAML/fulltest mismatch for `sigviewer` without rebuilding the image; it does not change the recipe's declared `architectures: [x86_64]`.

### Recipe-level full test check: `dcm2niix`

- On 2026-03-26, `./test.sh dcm2niix` was run against the existing local `dcm2niix:v1.0.20240202` image on an `aarch64` host without rebuilding the Docker image.
- Initial result: `43/105` tests passed, but that result was misleading.
- Cause:
  - `neurocontainers/recipes/dcm2niix/fulltest.yaml` appended `|| true` to many `dcm2niix` commands, which masked command failures and turned an unusable arm64 runtime into false-positive test passes
  - direct execution confirmed the real runtime error in the existing image:
    `/bin/sh: 1: dcm2niix: Exec format error`
- Fix landed in recipe YAML only: remove the `|| true` fallbacks from `neurocontainers/recipes/dcm2niix/fulltest.yaml` so command exit codes propagate normally during the full test run.
- Verified rerun result:
  - the same `./test.sh dcm2niix` invocation now fails cleanly with `0/105` tests passing in `25.2s`
  - the rerun consistently surfaces the underlying runtime problem instead of masking it behind shell fallbacks
- Current remaining blocker after this fix:
  - `neurocontainers/recipes/dcm2niix/build.yaml` downloads `dcm2niix_lnx.zip`, and the staged binary in the existing image is not executable on arm64
- Scope note: this closes a recipe YAML/fulltest masking issue for `dcm2niix`; it does not make the binary recipe arm64-ready.

### Recipe-level full test check: `hnncore`

- On 2026-03-26, `./test.sh hnncore` was run against the existing local `hnncore:0.3` image on an `aarch64` host without rebuilding the Docker image.
- Initial result: `66/68` tests passed. The two failures were in `neurocontainers/recipes/hnncore/fulltest.yaml`, not the container runtime broadly:
  `hnn-core version check` and `Create CellResponse object`.
- Cause:
  - the full test expected `hnn_core.__version__` to report `0.3`, but the shipped package in the existing image reports `0.5.0`
  - the full test used an outdated `CellResponse` constructor shape; the installed API requires `cell_type_names` as the first argument
- Fix landed in recipe YAML only:
  - update the expected HNN-core version string to `0.5.0`
  - update the `CellResponse` instantiation in `neurocontainers/recipes/hnncore/fulltest.yaml` to pass `cell_type_names` and keyword arguments matching the installed API
- Verified rerun result: the same `./test.sh hnncore` invocation then passed cleanly with `68/68` tests passing in `107.0s`.
- Scope note: this closes a recipe YAML/fulltest mismatch for `hnncore` without rebuilding the image; it does not change the recipe's declared `architectures: [x86_64]`.

### Recipe-level full test check: `dsistudio`

- On 2026-03-26, `./test.sh dsistudio` was run against the existing local `dsistudio:2024.06.12` image on an `aarch64` host without rebuilding the Docker image.
- Initial result: `24/83` tests passed, but that result was misleading.
- Cause:
  - `neurocontainers/recipes/dsistudio/fulltest.yaml` appended `|| true` to many `dsi_studio` commands, which masked command failures and made an unusable arm64 runtime look partially healthy
  - direct execution of the installed binary confirmed the underlying runtime problem:
    `/bin/sh: 1: dsi_studio: Exec format error`
- Fix landed in recipe YAML only: remove the `|| true` fallbacks from `neurocontainers/recipes/dsistudio/fulltest.yaml` so DSI Studio command exit codes propagate normally during the full test run.
- Verified rerun result:
  - the same `./test.sh dsistudio` invocation now fails with `6/83` tests passing in `22.8s`
  - the rerun consistently surfaces the real DSI Studio runtime failure (`126` / `Exec format error`) instead of converting it into false-positive passes
- Current remaining blockers after this fix:
  - the existing image's `dsi_studio` binary is not executable on arm64
  - several atlas/template path assertions also fail because the expected packaged files are not present in this runtime image
- Scope note: this closes a recipe YAML/fulltest masking issue for `dsistudio`; it does not make the recipe arm64-ready.

### Recipe-level full test check: `qupath`

- On 2026-03-26, `./test.sh qupath` was run against the existing local `qupath:0.6.0` image on an `aarch64` host without rebuilding the Docker image.
- Initial result: `3/120` tests passed, but the failure pattern was mostly noise.
- Cause:
  - the existing image's `QuPath` launcher is not runnable on this arm64 host and fails immediately with `Permission denied`
  - `neurocontainers/recipes/qupath/fulltest.yaml` had no early launcher preflight, so the suite continued into dozens of follow-on CLI/script tests that all failed for the same root cause
- Fix landed in recipe YAML only: add a setup-time `QuPath --version` preflight in `neurocontainers/recipes/qupath/fulltest.yaml` so the suite fails fast when the launcher itself cannot start.
- Verified rerun result:
  - the same `./test.sh qupath` invocation now fails immediately in setup with a single clear launcher failure (`Setup failed (exit 126)`)
  - the rerun reports `0/0` test cases instead of flooding the output with redundant downstream command failures
- Current remaining blocker after this fix:
  - the existing `qupath:0.6.0` image is still not runnable on arm64 because the `QuPath` launcher returns `Permission denied`
- Scope note: this closes a recipe YAML/fulltest signal-quality issue for `qupath`; it does not make the recipe arm64-ready.

### Recipe-level full test check: `mritools`

- On 2026-03-26, `./test.sh mritools` was run against the existing local `mritools:3.3.0` image on an `aarch64` host without rebuilding the Docker image.
- Initial result: `6/54` tests passed, but the failure pattern was mostly noise.
- Cause:
  - the existing image's `romeo`, `clearswi`, `mcpc3ds`, and `julia` launchers are not runnable on this arm64 host, so the suite reported dozens of follow-on `exit 126` failures from the same root cause
  - `neurocontainers/recipes/mritools/fulltest.yaml` had no early launcher preflight, so it continued into 48 derivative command failures after the first startup break
- Fix landed in recipe YAML only: add a setup-time `romeo --version` preflight in `neurocontainers/recipes/mritools/fulltest.yaml` so the suite fails immediately when the packaged executables cannot start.
- Verified rerun result:
  - the same `./test.sh mritools` invocation now fails immediately in setup with a single launcher failure (`Setup failed (exit 126)`)
  - the rerun reports `0/0` test cases instead of a misleading `6/54` result with 48 downstream failures
- Current remaining blocker after this fix:
  - a direct runtime probe with `apptainer exec sifs/mritools_3.3.0.simg romeo --version` still fails immediately on arm64 and prints:
    `/opt/mritools-3.3.0/bin/romeo: 1: ... ELF ... not found`
    followed by
    `Syntax error: "(" unexpected`
  - this indicates the packaged executables in the existing image are still incompatible with arm64
- Scope note: this closes a recipe YAML/fulltest signal-quality issue for `mritools`; it does not make the recipe arm64-ready.

### Recipe-level full test check: `laynii`

- On 2026-03-26, `./test.sh laynii` was run against the existing local `laynii:2.2.1` image on an `aarch64` host without rebuilding the Docker image.
- Initial result: `3/59` tests passed, but the failure pattern was mostly noise.
- Cause:
  - the existing image's LayNii executables are not runnable on this arm64 host, so the suite reported dozens of follow-on `exit 126` failures from the same startup problem
  - `neurocontainers/recipes/laynii/fulltest.yaml` had no early launcher preflight, so it continued into 56 derivative failures after the first command break
- Fix landed in recipe YAML only: add a setup-time `LN_INFO -help` preflight in `neurocontainers/recipes/laynii/fulltest.yaml` so the suite fails immediately when the packaged executables cannot start.
- Verified rerun result:
  - the same `./test.sh laynii` invocation now fails immediately in setup with a single launcher failure (`Setup failed (exit 126)`)
  - the rerun reports `0/0` test cases instead of a misleading `3/59` result with 56 downstream failures
- Current remaining blocker after this fix:
  - a direct runtime probe with `docker run --rm laynii:2.2.1 /bin/sh -lc 'LN_INFO -help 2>&1 | head -20'` fails immediately on arm64 with:
    `/bin/sh: 1: LN_INFO: Exec format error`
  - this indicates the packaged LayNii binaries in the existing image are still incompatible with arm64
- Scope note: this closes a recipe YAML/fulltest signal-quality issue for `laynii`; it does not make the recipe arm64-ready.

### Recipe-level full test check: `minc`

- On 2026-03-26, `./test.sh minc` was run against the existing local `minc:1.9.18` image on an `aarch64` host without rebuilding the Docker image.
- Initial result: `0/111` tests passed, but the failure pattern was mostly noise.
- Cause:
  - the existing image's MINC executables are not runnable on this arm64 host, so the suite reported one startup failure and then a long chain of dependent skips and follow-on failures
  - `neurocontainers/recipes/minc/fulltest.yaml` had no early launcher preflight, so the report expanded a single runtime incompatibility into 111 failed cases
- Fix landed in recipe YAML only: add a setup-time `mincinfo -version` preflight in `neurocontainers/recipes/minc/fulltest.yaml` so the suite fails immediately when the packaged executables cannot start.
- Verified rerun result:
  - the same `./test.sh minc` invocation now fails immediately in setup with a single launcher failure (`Setup failed (exit 126)`)
  - the rerun reports `0/0` test cases instead of the earlier misleading `0/111` failure cascade
- Current remaining blocker after this fix:
  - a direct runtime probe with `docker run --rm minc:1.9.18 /bin/sh -lc 'mincinfo -version 2>&1 | head -5'` fails immediately on arm64 with:
    `/bin/sh: 1: mincinfo: Exec format error`
  - this indicates the packaged MINC binaries in the existing image are still incompatible with arm64
- Scope note: this closes a recipe YAML/fulltest signal-quality issue for `minc`; it does not make the recipe arm64-ready.

### Template-level build check: `bids_validator/binaries`

- On 2026-03-26, `./build.sh bidscoin` on an `aarch64` host failed in the shared `bids_validator` template before `npm install` started.
- Initial failure:
  `E: Unable to locate package node_install_package`
- Cause: `pkg/recipe/template_macros/bids_validator__binaries.yaml` referenced `node_install_package` as a bare symbol inside `self.install(...)`, so the macro backend rendered the placeholder name literally into the Dockerfile.
- Follow-on issue uncovered while fixing the same step: the macro emitted a shell block that the builder joined with `&&`, producing `/bin/sh: Syntax error: "&&" unexpected` after the `fi`.
- Fix landed:
  - inline the package names directly inside the apt/yum branches instead of passing the unresolved placeholder through `self.install(...)`
  - keep the node version checks and `npm install` inside the same run block so the generated shell stays valid
- Verified results after the fix:
  - `go run ./cmd/builder template-tests bids-validator-binaries --build --run-tests` now gets past NodeSource setup, installs `nodejs`, prints `node --version` / `npm --version`, and reaches the real `npm install -g bids-validator@1.13.0`
  - the original `./build.sh bidscoin` run also gets past the same template regression and reaches the same `npm install` stage on arm64
- Current remaining blockers after this fix:
  - Ubuntu 22.04 template test path fails in `node-gyp` with `Error: not found: make`
  - Ubuntu 24.04 `bidscoin` path fails later in `node-gyp` with `ModuleNotFoundError: No module named 'distutils'`
- Scope note: this closes one builder/template regression in `bids_validator`; the template still is not arm64-ready end to end.

### `miniconda/binaries`

- The live template system is now the macro-backed implementation under `pkg/recipe/`.
- Template context exposes `self.arch`, so macro-backed templates can branch on architecture.
- `pkg/recipe/template_specs/miniconda.yaml` now uses `repo.anaconda.com` and `Linux-{{ self.arch }}` instead of hardcoding `x86_64`.
- `miniconda` supports an optional `installer_version` argument.
  This keeps modern recipes simple:
  `version: latest`
  while still allowing legacy artifact names such as:
  `version: 4.12.0`
  `installer_version: py37_4.12.0`
- `cmd/builder/template-tests` now supports `architecture: aarch64`.
- A dedicated arm64 template test entry now exists in `pkg/recipe/template_specs/test_all.yaml`.
- Focused unit tests were added for:
  - arm64 URL rendering
  - legacy installer filename override rendering

Notes:

- A generated arm64 Dockerfile correctly renders:
  `https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-aarch64.sh`
- A clean minimal arm64 container build completed and `conda --version` returned:
  `conda 26.1.1`
- Upstream availability is version-dependent.
  Confirmed present:
  - `Miniconda3-py37_4.12.0-Linux-aarch64.sh`
  - `Miniconda3-py38_22.11.1-1-Linux-aarch64.sh`
  - `Miniconda3-py310_25.5.1-0-Linux-aarch64.sh`
  Confirmed absent:
  - `Miniconda3-4.7.12-Linux-aarch64.sh`

## Template Usage By Method

| Template | Method | Unique Recipe Count |
|---|---|---:|
| `miniconda` | `binaries` | 39 |
| `matlabmcr` | `binaries` | 16 |
| `fsl` | `binaries` | 8 |
| `freesurfer` | `binaries` | 7 |
| `ants` | `binaries` | 4 |
| `ants` | `source` | 3 |
| `dcm2niix` | `binaries` | 3 |
| `mrtrix3` | `binaries` | 3 |
| `mrtrix3` | `source` | 3 |
| `convert3d` | `binaries` | 2 |
| `dcm2niix` | `source` | 2 |
| `spm12` | `binaries` | 2 |
| `afni` | `binaries` | 1 |
| `bids_validator` | `binaries` | 1 |
| `minc` | `binaries` | 1 |

## Recipes Using Confirmed-Broken Templates

### `miniconda/binaries` (39 recipes; builder template fixed, but some recipe versions may still need `installer_version`)

`amico`, `bidscoin`, `bidstools`, `blastct`, `brainlesion`, `cat12`, `clinica`, `clinicadl`, `code`, `condaenvs`, `connectomemapper3`, `dafne`, `deeplabcut`, `deepretinotopy`, `deepsif`, `eharmonize`, `esilpd`, `fitlins`, `fsqc`, `gouhfi`, `hcpasl`, `hdbet`, `megnet`, `mne`, `mrsimetabolicconnectome`, `neurocommand`, `nipype`, `palmettobug`, `pcntoolkit`, `pydeface`, `segmentator`, `slicer`, `soopct`, `spm12`, `spmpython`, `topaz`, `totalsegmentator`, `tractseg`, `vesselvio`

### `dcm2niix/binaries` (3 recipes)

`bidscoin`, `clinica`, `clinicadl`

### `convert3d/binaries` (2 recipes)

`braid`, `convert3d`

### `bids_validator/binaries` (1 recipe)

`bidscoin`

### `neurodebian/binaries`

No current recipes in `neurocontainers/recipes` directly use this built-in template.

### `ndfreeze/source`

No current recipes in `neurocontainers/recipes` directly use this built-in template.

## Recipes Already Using Working Source Templates

These are the best candidates for arm64 enablement once their remaining dependencies are verified.

### `dcm2niix/source` (2 recipes)

`bidstools`, `dicomtools`

### `mrtrix3/source` (3 recipes, still needs clean final close-out)

`braid`, `mrtrix3`, `mrtrix3tissue`

### `ants/source` (3 recipes, still needs clean final close-out)

`ants`, `braid`, `nighres`

## Non-Ubuntu Base Images

These recipes do not use an `ubuntu:*` base image and should be considered separately for arm64 pull testing.

The reusable checker for this is:

- `scripts/check_arm64_base_images.py`

It scans recipe roots from `builder.config.yaml`, finds non-Ubuntu bases, and attempts:

- `docker pull --platform linux/arm64 <image>`

### Non-Ubuntu Recipes That Also Use Templates

| Recipe | Base Image | Template Usage | Arm64 Pull Check |
|---|---|---|---|
| `code` | `debian:bookworm` | `miniconda/binaries` | `ok` |
| `connectomeworkbench` | `neurodebian:bookworm-non-free` | `freesurfer/binaries` | `ok` |
| `convert3d` | `debian:bookworm` | `convert3d/binaries` | `ok` |
| `deepretinotopy` | `ghcr.io/neurodesk/freesurfer_7.3.2:20230216` | `miniconda/binaries` | `ok` |
| `deepsif` | `debian:11` | `miniconda/binaries` | `timeout` after 45s |
| `esilpd` | `debian:11` | `miniconda/binaries` | `timeout` after 45s |
| `mrtrix3tissue` | `ghcr.io/neurodesk/caid/fsl_6.0.3:20200905` | `mrtrix3/source` | `amd64 only` |
| `pydeface` | `vnmd/fsl_6.0.3:20200905` | `miniconda/binaries` | `amd64 only` |

Interpretation:

- `ok` means the image is directly pullable as `linux/arm64`.
- `timeout` does not prove failure.
  It only means the pull did not finish within the short probe window used for this audit pass.
- `amd64 only` means manifest/config inspection showed the image is not arm64-native.

## Custom Base Images

Definition used here:

- a `custom` base image is any recipe `base-image` that is not one of the plain distro/root images `ubuntu`, `debian`, `centos`, `fedora`, `rockylinux`, or `python`

Classification method:

- manifest/config inspection using `docker buildx imagetools inspect`
- `unresolved` means the recipe tag still contains `{{ ... }}`
- `inspect_failed` means the registry could not be classified in this pass, usually due to Docker Hub rate limiting, a missing tag, or an unsupported legacy manifest format

### Custom Image Summary

| Status | Unique Images |
|---|---:|
| `amd64_only` | 26 |
| `arm64+amd64` | 3 |
| `unresolved` | 7 |
| `inspect_failed` | 13 |

### Custom Images With Native Arm64 Support

| Image | Architectures | Recipes |
|---|---|---|
| `docker.io/nvidia/cuda:12.0.0-devel-ubuntu22.04` | `amd64`, `arm64` | `bart` |
| `moby/buildkit:latest` | `amd64`, `arm/v7`, `arm64`, `ppc64le`, `riscv64`, `s390x` | `builder` |
| `quay.io/jupyter/minimal-notebook` | `amd64`, `arm64` | `irkernel` |

### Custom Images Confirmed `amd64_only`

| Image | Recipes |
|---|---|
| `bids/baracus:v1.1.4` | `bidsappbaracus` |
| `bids/brainsuite:v21a` | `bidsappbrainsuite` |
| `bids/giga_connectome:0.6.0` | `gigaconnectome` |
| `bids/hcppipelines:v4.3.0-3` | `bidsapphcppipelines` |
| `bids/mrtrix3_connectome:0.5.3` | `bidsappmrtrix3connectome` |
| `bids/pymvpa:v2.0.2` | `bidsapppymvpa` |
| `bids/spm:v0.0.20` | `bidsappspm` |
| `bradley987/bidsme:1.9.3` | `bidsme` |
| `dcanumn/osprey-bids:v4.2.1` | `ospreybids` |
| `dmri/neurodock:v1.0.0` | `neurodock` |
| `docker.io/tensorflow/tensorflow:1.15.0-gpu-py3` | `delphi` |
| `dorianps/lesymap:20220701` | `lesymap` |
| `dorianps/linda` | `linda` |
| `exploreasl/xasl:1.11.0` | `exploreasl` |
| `fcpindi/c-pac:release-v1.8.7.post1.dev3` | `cpac` |
| `freesurfer/synthstrip:1.6` | `quickshear` |
| `freesurfer/synthstrip:1.8` | `syncro` |
| `ghcr.io/farwa-abbas/nftsim:1.0.2` | `nftsim` |
| `ghcr.io/metaphorme/vina-all:release` | `vina` |
| `ghcr.io/neurodesk/caid/fsl_6.0.3:20200905` | `mrtrix3tissue` |
| `ghcr.io/neurodesk/freesurfer_7.3.2:20230216` | `deepretinotopy` |
| `halfpipe/halfpipe:1.2.3` | `halfpipe` |
| `jerync/oshyx_0.4:20220614` | `oshyx` |
| `pennlinc/aslprep:0.7.5` | `aslprep` |
| `pennlinc/xcp_d:0.10.7` | `xcpd` |
| `pytorch/pytorch:2.4.1-cuda11.8-cudnn9-runtime` | `musclemap`, `spinalcordtoolbox`, `vesselboost` |

Notes:

- `vnmd/fsl_6.0.3:20200905` is also `amd64` only.
  That was confirmed separately by pulling the image config and inspecting `.Architecture`.
- `ghcr.io/neurodesk/caid/fsl_6.0.3:20200905` and `vnmd/fsl_6.0.3:20200905` resolve to the same digest and are both `amd64`.

### Custom Images With Unresolved Templated Tags

| Image | Recipes |
|---|---|
| `deepmi/fastsurfer:cpu-v{{ context.version }}` | `fastsurfer` |
| `ghcr.io/spm/spm-docker:docker-matlab-{{ context.version }}` | `spm25` |
| `ghcr.io/tinyrange/tinyrange:v{{ context.version }}` | `tinyrange` |
| `nipreps/fmriprep:{{ context.version }}` | `fmriprep` |
| `pennlinc/qsiprep:{{ context.version }}` | `qsiprep` |
| `pennlinc/qsirecon:{{ context.version }}` | `qsirecon` |
| `unfmontreal/dcm2bids:{{ context.version }}` | `dcm2bids` |

### Custom Images Not Classified Cleanly In This Pass

| Image | Reason | Recipes |
|---|---|---|
| `bids/aa:v0.2.0` | legacy schema v1 manifest unsupported by `imagetools` | `bidsappaa` |
| `jamovi/jamovi:2.3.17` | tag not found | `jamovi` |
| `sljhlab/openads:1.0.0-gpu` | tag not found | `openads` |
| `kytk/batch-heudiconv` | Docker Hub `429 Too Many Requests` | `batchheudiconv` |
| `micalab/micapipe:v0.2.3` | Docker Hub `429 Too Many Requests` | `micapipe` |
| `neurodebian:bookworm-non-free` | Docker Hub `429 Too Many Requests` | `connectomeworkbench`, `datalad`, `template` |
| `neurodebian:bullseye` | Docker Hub `429 Too Many Requests` | `sigviewer` |
| `neurodebian:nd20.04-non-free` | Docker Hub `429 Too Many Requests` | `ezbids` |
| `nipreps/mriqc:24.0.2` | Docker Hub `429 Too Many Requests` | `mriqc` |
| `nipreps/nibabies:24.0.0` | Docker Hub `429 Too Many Requests` | `nibabies` |
| `nipy/heudiconv:1.3.1` | Docker Hub `429 Too Many Requests` | `heudiconv` |
| `rootproject/root:6.22.02-centos7` | Docker Hub `429 Too Many Requests` | `root` |
| `vnmd/fsl_6.0.3:20200905` | Docker Hub `429 Too Many Requests` during manifest scan, but separately confirmed `amd64` only | `pydeface` |

### Non-Ubuntu Base Image Frequency

| Count | Base Image |
|---|---|
| 4 | `centos:7` |
| 3 | `neurodebian:bookworm-non-free` |
| 3 | `pytorch/pytorch:2.4.1-cuda11.8-cudnn9-runtime` |
| 2 | `debian:11` |
| 2 | `debian:bookworm` |
| 1 | `bids/aa:v0.2.0` |
| 1 | `bids/baracus:v1.1.4` |
| 1 | `bids/brainsuite:v21a` |
| 1 | `bids/giga_connectome:0.6.0` |
| 1 | `bids/hcppipelines:v4.3.0-3` |
| 1 | `bids/mrtrix3_connectome:0.5.3` |
| 1 | `bids/pymvpa:v2.0.2` |
| 1 | `bids/spm:v0.0.20` |
| 1 | `bradley987/bidsme:1.9.3` |
| 1 | `dcanumn/osprey-bids:v4.2.1` |
| 1 | `deepmi/fastsurfer:cpu-v{{ context.version }}` |
| 1 | `dmri/neurodock:v1.0.0` |
| 1 | `docker.io/nvidia/cuda:12.0.0-devel-ubuntu22.04` |
| 1 | `docker.io/tensorflow/tensorflow:1.15.0-gpu-py3` |
| 1 | `dorianps/lesymap:20220701` |
| 1 | `dorianps/linda` |
| 1 | `exploreasl/xasl:1.11.0` |
| 1 | `fcpindi/c-pac:release-v1.8.7.post1.dev3` |
| 1 | `fedora:35` |
| 1 | `freesurfer/synthstrip:1.6` |
| 1 | `freesurfer/synthstrip:1.8` |
| 1 | `ghcr.io/farwa-abbas/nftsim:1.0.2` |
| 1 | `ghcr.io/metaphorme/vina-all:release` |
| 1 | `ghcr.io/neurodesk/caid/fsl_6.0.3:20200905` |
| 1 | `ghcr.io/neurodesk/freesurfer_7.3.2:20230216` |
| 1 | `ghcr.io/spm/spm-docker:docker-matlab-{{ context.version }}` |
| 1 | `ghcr.io/tinyrange/tinyrange:v{{ context.version }}` |
| 1 | `halfpipe/halfpipe:1.2.3` |
| 1 | `jamovi/jamovi:2.3.17` |
| 1 | `jerync/oshyx_0.4:20220614` |
| 1 | `kytk/batch-heudiconv` |
| 1 | `micalab/micapipe:v0.2.3` |
| 1 | `moby/buildkit:latest` |
| 1 | `neurodebian:bullseye` |
| 1 | `neurodebian:nd20.04-non-free` |
| 1 | `nipreps/fmriprep:{{ context.version }}` |
| 1 | `nipreps/mriqc:24.0.2` |
| 1 | `nipreps/nibabies:24.0.0` |
| 1 | `nipy/heudiconv:1.3.1` |
| 1 | `pennlinc/aslprep:0.7.5` |
| 1 | `pennlinc/qsiprep:{{ context.version }}` |
| 1 | `pennlinc/qsirecon:{{ context.version }}` |
| 1 | `pennlinc/xcp_d:0.10.7` |
| 1 | `python:3.12.0-slim` |
| 1 | `quay.io/jupyter/minimal-notebook` |
| 1 | `rootproject/root:6.22.02-centos7` |
| 1 | `sljhlab/openads:1.0.0-gpu` |
| 1 | `unfmontreal/dcm2bids:{{ context.version }}` |
| 1 | `vnmd/fsl_6.0.3:20200905` |

## Suggested Next Steps

1. Finish clean arm64 verification for `ants/source`, `mrtrix3/source`, and `fsl/binaries`.
2. Treat `matlabmcr/binaries` as x86_64-only unless upstream changes.
3. Decide whether `dcm2niix/binaries` and `convert3d/binaries` should gain arm64-specific URLs or whether recipes should migrate to source builds.
4. Use `scripts/check_arm64_base_images.py` for the full non-Ubuntu arm64 pull sweep, then fold the results back into this document.
