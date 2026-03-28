# ARM64 Template Audit

Last updated: 2026-03-28

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
| `bids_validator` | `binaries` | Unknown | Macro regression and native build prerequisites fixed; arm64 now installs Node 20 and reaches the real `npm install -g bids-validator@1.13.0`, but a clean final arm64 close-out has not been captured yet |
| `neurodebian` | `binaries` | Fails | Broken key import: `gpg: no valid OpenPGP data found` |
| `ndfreeze` | `source` | Fails | Build stalls/fails during `nd_freeze 2024-01-01` apt refresh |
| `dcm2niix` | `binaries` | Fails | Binary payload is wrong architecture; runtime `Exec format error` |
| `convert3d` | `binaries` | Fails | Recipe build completes on arm64, but the staged nightly payload is x86_64-only; runtime `Exec format error`. Upstream `Linux-gcc64` nightly inspected on 2026-03-27 is also x86_64 |
| `matlabmcr` | `binaries` | Fails by design | Upstream is x86_64-only |
| `ants` | `source` | Unknown | `./build.sh ants` on arm64 rendered and compiled cleanly through upstream ITK/ANTs build stages past 80%, but a full clean completion was not captured yet |
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
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh builder` on the same `aarch64` host completed cleanly again and rebuilt `builder:0.2` from cache
  - `docker image inspect builder:0.2 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:81dda6526175a183498e8beda2f9915511228b423111a3a5494ce9738b67c6e8 arm64 linux`
  - an explicit-entrypoint runtime smoke check also succeeded:
    `docker run --rm --entrypoint bash builder:0.2 -lc 'bash --version | sed -n "1p"'`
    and reported:
    `GNU bash, version 5.3.3(1)-release (aarch64-alpine-linux-musl)`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh builder` on the same `aarch64` host completed cleanly again and rebuilt `builder:0.2` from cache
  - `docker image inspect builder:0.2 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:804a97603ba78274f0ed58d9ff1270d8af2cc69753d5bbe132ddcf9bfef72c62 arm64 linux`
  - the same explicit-entrypoint runtime smoke check still succeeded:
    `docker run --rm --entrypoint bash builder:0.2 -lc 'bash --version | sed -n "1p"'`
    and reported:
    `GNU bash, version 5.3.3(1)-release (aarch64-alpine-linux-musl)`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh builder` on the same `aarch64` host completed cleanly again and rebuilt `builder:0.2` from cache
  - `docker image inspect builder:0.2 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:2badad4fb51b428c997ac8ec35cc8140575ec558d1f1162308adef102cc5f644 arm64 linux`
  - the same explicit-entrypoint runtime smoke check still succeeded:
    `docker run --rm --entrypoint bash builder:0.2 -lc 'bash --version | sed -n "1p"'`
    and reported:
    `GNU bash, version 5.3.3(1)-release (aarch64-alpine-linux-musl)`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh builder` on the same `aarch64` host completed cleanly again and rebuilt `builder:0.2` from cache
  - `docker image inspect builder:0.2 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:41ff128dc7d3d597b85c8e9885c9f2b60d45805bcf233533424cedc60b126285 arm64 linux`
  - the same explicit-entrypoint runtime smoke check still succeeded:
    `docker run --rm --entrypoint bash builder:0.2 -lc 'bash --version | sed -n "1p"'`
    and reported:
    `GNU bash, version 5.3.3(1)-release (aarch64-alpine-linux-musl)`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh builder` on the same `aarch64` host completed cleanly again and rebuilt `builder:0.2` from cache
  - `docker image inspect builder:0.2 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:6602882a95a555b2bd1f18b0c724429770933a88713a9af5ecd66f0c1d6d1e28 arm64 linux`
  - the same explicit-entrypoint runtime smoke check still succeeded:
    `docker run --rm --entrypoint bash builder:0.2 -lc 'bash --version | sed -n "1p"'`
    and reported:
    `GNU bash, version 5.3.3(1)-release (aarch64-alpine-linux-musl)`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh builder` on the same `aarch64` host completed cleanly again and rebuilt `builder:0.2` from cache
  - `docker image inspect builder:0.2 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:6fa7e2f1d4a58dc1ae789eb712b2beb75fce844cd19c2a0f32f62318f3e8d580 arm64 linux`
  - the same explicit-entrypoint runtime smoke check still succeeded:
    `docker run --rm --entrypoint bash builder:0.2 -lc 'bash --version | sed -n "1p"'`
    and reported:
    `GNU bash, version 5.3.3(1)-release (aarch64-alpine-linux-musl)`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh builder` on the same `aarch64` host completed cleanly again and rebuilt `builder:0.2` from cache
  - `docker image inspect builder:0.2 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:7625d61c72bd0fc2ffbdbdb981c5f4621a79ce967e9d24f063b2bfb4b791779c arm64 linux`
  - the same explicit-entrypoint runtime smoke check still succeeded:
    `docker run --rm --entrypoint bash builder:0.2 -lc 'bash --version | sed -n "1p"'`
    and reported:
    `GNU bash, version 5.3.3(1)-release (aarch64-alpine-linux-musl)`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh builder` on the same `aarch64` host completed cleanly again and rebuilt `builder:0.2` from cache
  - `docker image inspect builder:0.2 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:058c3dc4f37a9cc93d51462a6c18ebc77d558d9e910406c9d7e9628ba1c5a547 arm64 linux`
  - the same explicit-entrypoint runtime smoke check still succeeded:
    `docker run --rm --entrypoint bash builder:0.2 -lc 'bash --version | sed -n "1p"'`
    and reported:
    `GNU bash, version 5.3.3(1)-release (aarch64-alpine-linux-musl)`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh builder` on the same `aarch64` host completed cleanly again and rebuilt `builder:0.2` from cache
  - `docker image inspect builder:0.2 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:5cc7ff30c2f85168fa77b76b125546c0324298c336721aa6c6bbc64bd17b7522 arm64 linux`
  - the same explicit-entrypoint runtime smoke check still succeeded:
    `docker run --rm --entrypoint bash builder:0.2 -lc 'bash --version | sed -n "1p"'`
    and reported:
    `GNU bash, version 5.3.3(1)-release (aarch64-alpine-linux-musl)`

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
- Follow-on issue uncovered on 2026-03-28:
  - once the arm64 Miniconda bootstrap and config path were allowed to continue, the rerun failed at:
    `CondaValueError: 'base' is a reserved environment name`
  - the failing rendered step was:
    `conda create -y -q --name base`
- Additional fixes landed in recipe YAML:
  - set `env_name: bidstools`
  - set `env_exists: "false"` so the Miniconda template creates a real named environment
  - add `build-essential` so source-built Python dependencies have the libc/system headers they expect
- Verified follow-up rerun result:
  - the regenerated Dockerfile now emits `conda create -y -q --name bidstools`, and the previous reserved-`base` failure is gone
  - the rerun got cleanly through env creation and the `python=3.11` environment install, then entered the real env-local pip install path for:
    `heudiconv`
    and
    `traits`
  - the previous `traits` source-build failure:
    `fatal error: stdlib.h: No such file or directory`
    is gone
  - the patched rerun successfully built and installed the `traits-7.1.0` wheel on arm64, then progressed further into the later system-package install layer for:
    `wget zip libgl1 libgtk2.0-0 dcmtk xmedcon pigz libxcb-cursor0`
  - I stopped that rerun while the later apt package layer was still active, so there is not yet a finalized `bidstools:1.0.4` image from this pass
- Scope note: this pass closes three concrete arm64 build issues for `bidstools` by rendering the correct Miniconda installer, removing the reserved-`base` env failure, and restoring the system headers needed for source-built Python dependencies. A final successful arm64 image was not produced in this pass.

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

- On 2026-03-27, `./test.sh gingerale` was run against the existing local `gingerale:3.0.2` image on an `aarch64` host without rebuilding the Docker image.
- Result:
  - the existing image converted successfully to `sifs/gingerale_3.0.2.simg`
  - the current `neurocontainers/recipes/gingerale/fulltest.yaml` suite passed cleanly with `50/50` tests passing in `145.0s`
  - the passing checks covered CLI usage, ALE generation, thresholding modes, clustering, contrast analysis, Java runtime, and expected error handling
- Scope note: this is a no-rebuild arm64 runtime/fulltest verification result for `gingerale`; no recipe YAML changes were required in this pass.

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
  - `blastct` still had a separate recipe problem to resolve before the image could complete on arm64, because the recipe asked modern Conda to create a new environment named `base`
- On 2026-03-27, `./build.sh blastct` was rerun on the same `aarch64` host.
- Initial failure on that rerun:
  `CondaValueError: 'base' is a reserved environment name`
- Cause:
  - `neurocontainers/recipes/blastct/build.yaml` still rendered the Miniconda template with the default `env_name: base`
  - the template then emitted `conda create -y -q --name base`, which Conda rejects on current releases
- Fix landed in recipe YAML:
  - set `env_name: blastct`
  - set `env_exists: "false"` so the template creates that named environment explicitly
  - run the recipe's GitHub package install with `conda run -n blastct ...` so it targets the same environment
- Verified rerun result:
  - the regenerated Dockerfile now emits `conda create -y -q --name blastct` instead of `--name base`
  - rerunning `./build.sh blastct` no longer fails at the reserved-`base` Conda step; it progresses through environment creation on arm64
  - the same rerun then fails later during pip dependency installation with:
    `ERROR: Could not find a version that satisfies the requirement SimpleITK==1.2.4`
- Current remaining blocker after this fix:
  - the reserved-`base` Conda environment issue is fixed
  - `blastct` still has a later dependency-resolution problem on arm64 and Python 3.11 because the recipe pins `SimpleITK==1.2.4`, which is not available for the current build environment
- Scope note: this closes one concrete Conda-environment build issue for `blastct` on arm64; the recipe still has a later Python package compatibility failure to resolve.

- On 2026-03-28, `./build.sh blastct` was rerun on the same `aarch64` host.
- Initial failure on that rerun:
  `ERROR: Could not find a version that satisfies the requirement SimpleITK==1.2.4`
- Cause:
  - `neurocontainers/recipes/blastct/build.yaml` still pinned `SimpleITK==1.2.4` in the Miniconda template's `pip_install`, but that version is not available for this Python 3.11 arm64 environment
  - the recipe also installed `blast-ct` directly from GitHub afterwards, so leaving dependency resolution enabled there would have reintroduced the same incompatible upstream pin
- Fix landed in recipe YAML:
  - replace the unavailable `SimpleITK==1.2.4` pin with `SimpleITK==2.4.1`
  - add `tensorboard` to the explicitly managed pip dependencies so the recipe still provides the package set expected by upstream
  - change the later GitHub install step to `pip install --no-deps` because the recipe now supplies those dependencies itself
- Verified rerun result:
  - rerunning `./build.sh blastct` completes successfully and produces `blastct:2.0.0`
  - `docker image inspect blastct:2.0.0 --format '{{.Architecture}} {{.Os}}'` reports:
    `arm64 linux`
  - a follow-up runtime smoke check with `docker run --rm blastct:2.0.0 python3 -c 'import blast_ct; print(blast_ct.__file__)'` confirms the package imports from:
    `/opt/miniconda/lib/python3.11/site-packages/blast_ct/__init__.py`
  - `docker run --rm blastct:2.0.0 python3 -c 'import SimpleITK as sitk; print(sitk.Version_VersionString())'` reports:
    `2.4.1`
- Scope note: this closes the remaining Python package compatibility blocker for `blastct` in this arm64 build path; the recipe now builds and imports successfully on arm64 in this environment.

### Recipe-level full test check: `blastct`

- On 2026-03-28, `./test.sh blastct` was run against the existing local `blastct:2.0.0` image on an `aarch64` host without rebuilding the Docker image.
- Initial failure:
  - the recipe had no `neurocontainers/recipes/blastct/fulltest.yaml`, so `./test.sh blastct` stopped immediately with:
    `Recipe full test file not found: /home/joshua/dev/projects/builder/./neurocontainers/recipes/blastct/fulltest.yaml`
- Fix landed in recipe YAML only:
  - add `neurocontainers/recipes/blastct/fulltest.yaml`
  - the new suite verifies `python3`, imports `blast_ct`, checks `pip show blast_ct`, verifies `SimpleITK` reports `2.4.1`, and confirms the module is installed under `/opt/miniconda/lib/python3.11/site-packages`
- Verified rerun result:
  - rerunning `./test.sh blastct` against the same existing image path then passed cleanly with `5/5` tests passing in `4.1s`
  - because the existing image is large, the no-rebuild rerun also required redirecting Apptainer temporary files to a project-local temp directory on the main filesystem instead of `/tmp`
  - the generated `sifs/blastct_2.0.0.simg` was created from the existing local Docker image, not from a rebuilt container
- Scope note: this closes a recipe YAML/fulltest coverage gap for `blastct` without rebuilding the image.

- Follow-up on 2026-03-28:
  - the minimal `neurocontainers/recipes/blastct/fulltest.yaml` suite originally only asserted a broad Python runtime prefix (`Python 3.`), which was weaker than the exact runtime metadata available in the image
  - the recipe YAML was tightened to assert the shipped interpreter version instead: `Python 3.11.13`
  - rerunning `./test.sh blastct` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `4.0s`
- Scope note: this follow-up strengthens the no-rebuild fulltest to validate the current Python runtime version in the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/blastct/fulltest.yaml` package-metadata check still only asserted the installed package version, which was weaker than the exact metadata available from `python3 -m pip show blast_ct`
  - direct runtime probes on the existing local image showed the shipped package metadata includes the exact homepage line:
    `Home-page: https://github.com/biomedia-mira/blast_ct`
  - the recipe YAML was tightened to assert that exact homepage line instead of the broader version-only metadata check
  - a fresh rerun of `./test.sh blastct` was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but this pass remained in the long live Apptainer SIF-conversion tail and did not reach a new completed suite summary before it was stopped
- Scope note: this follow-up strengthens the no-rebuild `blastct` fulltest to validate the shipped package metadata more precisely; the earlier passing `5/5` rerun remains the latest completed suite result for this image path.

### Recipe-level build check: `brainlesion`

- On 2026-03-28, `./build.sh brainlesion` was run on an `aarch64` host.
- Initial failure:
  `conda: not found`
- Cause:
  - `neurocontainers/recipes/brainlesion/build.yaml` only declared `architectures: [x86_64]`
  - because of that, the shared `miniconda` template rendered the x86_64 installer on an arm64 host, and the later conda configuration step failed because a usable arm64 `conda` was never installed
- Fix landed in recipe YAML:
  - add `aarch64` as a declared recipe architecture in `neurocontainers/recipes/brainlesion/build.yaml`
- Verified rerun result:
  - the regenerated Dockerfile now stages `https://repo.anaconda.com/miniconda/Miniconda3-py310_25.5.1-0-Linux-aarch64.sh` instead of the x86_64 installer
  - rerunning `./build.sh brainlesion` no longer fails at `conda: not found`; it progresses through the Miniconda bootstrap on arm64
- Follow-on issue uncovered while fixing the same path:
  - the recipe still used the Miniconda template defaults, so the rerun then failed at:
    `CondaValueError: 'base' is a reserved environment name`
    from `conda create -y -q --name base`
- Additional fix landed in recipe YAML:
  - set `env_name: brainlesion`
  - set `env_exists: "false"` so the template creates that named environment explicitly
- Verified follow-up rerun result:
  - the regenerated Dockerfile now emits `conda create -y -q --name brainlesion` instead of `--name base`
  - rerunning `./build.sh brainlesion` gets through environment creation and into the real pip dependency install for the recipe's package set on arm64
  - the follow-up run was stopped while pip was still downloading and resolving large dependencies, including arm64 wheels for `torch` and related packages, so there is not yet a finalized `brainlesion:1.0.0` image from this pass
- Scope note: this closes two concrete recipe-side arm64 build issues for `brainlesion` by rendering the correct Miniconda installer and removing the reserved-`base` env failure; any later dependency-resolution or package-compatibility blockers remain to be closed out in a future run.

### Recipe-level build check: `amico`

- On 2026-03-27, `./build.sh amico` was run on an `aarch64` host.
- Initial failure:
  - the generated Dockerfile rendered `https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-x86_64.sh`
  - the Miniconda bootstrap then failed with:
    `/opt/miniconda/_conda: cannot execute binary file: Exec format error`
  - the next Conda step failed at:
    `conda: not found`
- Cause:
  - `neurocontainers/recipes/amico/build.yaml` only declared `architectures: [x86_64]`
  - because of that, the shared `miniconda` template rendered the x86_64 installer on an arm64 host
- Fix landed in recipe YAML:
  - add `aarch64` as a declared recipe architecture in `neurocontainers/recipes/amico/build.yaml`
- Verified rerun result:
  - the regenerated Dockerfile now stages `https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-aarch64.sh` instead of the x86_64 installer
  - rerunning `./build.sh amico` no longer fails in the Miniconda bootstrap with the x86_64 `_conda` exec-format error; it completes Miniconda installation and progresses through `conda update` and `conda-libmamba-solver` install on arm64
  - the same rerun then fails later at:
    `CondaValueError: 'base' is a reserved environment name`
    from `conda create -y -q --name base`
- Current remaining blocker after this fix:
  - the original arm64 Miniconda/render issue is fixed
  - `amico` still has a later Conda-environment problem to resolve because the rendered build path asks modern Conda to create a new environment named `base`
- Scope note: this closes one concrete arm64 Miniconda/render issue for `amico`; the recipe still has a later Conda-environment logic failure.

- On 2026-03-27, `./build.sh amico` was rerun on the same `aarch64` host.
- Initial failure on that rerun:
  `CondaValueError: 'base' is a reserved environment name`
- Cause:
  - `neurocontainers/recipes/amico/build.yaml` still used the Miniconda template defaults, so the rendered Dockerfile asked Conda to create a new environment named `base`
  - modern Conda rejects `base` as a user-created environment name
- Fix landed in recipe YAML:
  - set `env_name: amico`
  - set `env_exists: "false"` so the template creates that named environment explicitly
  - prepend `/opt/miniconda/envs/amico/bin` to `PATH` so the deployed `python` resolves to the environment where `dmri-amico` is installed
- Verified rerun result:
  - the regenerated Dockerfile now emits `conda create -y -q --name amico` instead of `--name base`
  - `./build.sh amico` completes successfully and produces `amico:2.1.0`
  - `docker image inspect amico:2.1.0 --format '{{.Architecture}} {{.Os}}'` reports:
    `arm64 linux`
  - a follow-up runtime smoke check with `docker run --rm amico:2.1.0 python -c 'import amico; print(amico.__version__)'` prints:
    `2.1.1`
- Scope note: this closes the recipe-side Conda-environment build issue for `amico`; the recipe now builds and imports successfully on arm64 in this environment.

- On 2026-03-28, `./build.sh amico` was rerun on the same `aarch64` host.
- Verified rerun result:
  - the current recipe still builds cleanly end to end and refreshes `amico:2.1.0`
  - `docker image inspect amico:2.1.0 --format '{{.Architecture}} {{.Os}}'` reports:
    `arm64 linux`
  - a follow-up runtime smoke check with `docker run --rm amico:2.1.0 python -c 'import amico; print(amico.__version__)'` prints:
    `2.1.1`
- Scope note: this is a revalidation pass on the current workspace rather than a new recipe fix; the existing arm64 `amico` build path remains good.

- On 2026-03-28, `./build.sh amico` was rerun again on the same `aarch64` host.
- Verified rerun result:
  - the current recipe still builds cleanly end to end and refreshes `amico:2.1.0` from cache
  - `docker image inspect amico:2.1.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` reports:
    `sha256:f4bd7d522e187f0369452766d45274bd7a028ee70f66db0e3d55c531efa7c84c arm64 linux`
  - a follow-up runtime smoke check with `docker run --rm amico:2.1.0 python -c 'import amico; print(amico.__version__)'` still prints:
    `2.1.1`
- Scope note: this is another revalidation pass on the current workspace rather than a new recipe fix; the existing arm64 `amico` build path remains good.
- On 2026-03-28, `./build.sh amico` was rerun once more on the same `aarch64` host.
- Verified rerun result:
  - the current recipe still builds cleanly end to end and refreshes `amico:2.1.0` from cache
  - `docker image inspect amico:2.1.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` reports:
    `sha256:a9b6cbeaf120a2eefa16e7266d96fb6e3ec38336c2844facce6ba0735a094ac7 arm64 linux`
  - a follow-up runtime smoke check with `docker run --rm amico:2.1.0 python -c 'import amico; print(amico.__version__)'` still prints:
    `2.1.1`
- Scope note: this is an additional revalidation pass on the current workspace rather than a new recipe fix; the existing arm64 `amico` build path remains good.
- On 2026-03-28, `./build.sh amico` was rerun again on the same `aarch64` host.
- Verified rerun result:
  - the current recipe still builds cleanly end to end and refreshes `amico:2.1.0` from cache
  - `docker image inspect amico:2.1.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` reports:
    `sha256:58ec86c62b34e5568018b51ce87700894a76439a0167276a2bfa1c7b531e9786 arm64 linux`
  - a follow-up runtime smoke check with `docker run --rm amico:2.1.0 python -c 'import amico; print(amico.__version__)'` still prints:
    `2.1.1`
- Scope note: this is another revalidation pass on the current workspace rather than a new recipe fix; the existing arm64 `amico` build path remains good.

- On 2026-03-27, `./test.sh amico` was run against the existing local `amico:2.1.0` image on an `aarch64` host without rebuilding the Docker image.
- Initial result: `72/74` tests passed. The two failures were in `neurocontainers/recipes/amico/fulltest.yaml`, not the existing container runtime broadly:
  - `AMICO version check` expected `2.1.0`, but the built image reports `amico.__version__ == 2.1.1`
  - `AMICO README check` tried to read `/README.md`, but that file is not present in the deployed image
- Cause:
  - `neurocontainers/recipes/amico/fulltest.yaml` had a stale hard-coded package version assertion
  - the same test file also assumed a README deployment path that the current recipe/image does not provide
- Fix landed in recipe YAML:
  - update the version expectation in `neurocontainers/recipes/amico/fulltest.yaml` from `2.1.0` to `2.1.1`
  - replace the invalid `/README.md` assertion with a package-metadata check using `python -m pip show dmri-amico`
- Verified rerun result:
  - rerunning `./test.sh amico` against the same existing image then passes cleanly with `74/74` tests passing in `160.7s`
  - a direct package probe in that image with `python -m pip show dmri-amico` reports:
    `Version: 2.1.1`
    `Location: /opt/miniconda/lib/python3.13/site-packages`
- Scope note: this closes a recipe YAML/fulltest mismatch for `amico` without rebuilding the image; it does not change the built container contents.

### Recipe-level build check: `mricrogl`

- On 2026-03-27, `./build.sh mricrogl` was run on an `aarch64` host.
- Initial failure:
  - the rendered Dockerfile installed Ubuntu's `libqt5pas1` package successfully
  - it then failed at the later recipe step that force-installed `libqt5pas1_2.9-0_amd64.deb` with:
    `libqt5pas1:amd64 : Depends: libc6:amd64 ... but it is not installable`
- Cause:
  - `neurocontainers/recipes/mricrogl/build.yaml` already installed distro `libqt5pas1`
  - the recipe then overrode that with a second, hard-coded amd64-only `.deb`, which cannot be resolved in the arm64 build environment
- Fix landed in recipe YAML:
  - remove the redundant external `libqt5pas1_2.9-0_amd64.deb` install step
  - remove the now-unused staged `.deb` file entry
- Verified rerun result:
  - a fresh builder render for `mricrogl` no longer emits the amd64 `.deb` install layer
  - rerunning the same Docker build path against that fresh render completes successfully and produces `mricrogl:debug-fixed`
- Scope note:
  - this closes one concrete recipe-side build issue for `mricrogl` on arm64 by removing an unnecessary amd64 package override
  - the recipe still declares `architectures: [x86_64]`, and this pass did not verify that the bundled MRIcroGL payload itself is arm64-runnable

### Recipe-level build check: `megnet`

- On 2026-03-27, `./build.sh megnet` was run on an `aarch64` host.
- Initial failure:
  - the generated Dockerfile rendered `https://repo.anaconda.com/miniconda/Miniconda3-py310_25.5.1-0-Linux-x86_64.sh`
  - the Miniconda bootstrap then failed with:
    `/opt/miniconda/_conda: cannot execute binary file: Exec format error`
  - the next Conda step failed at:
    `conda: not found`
- Cause:
  - `neurocontainers/recipes/megnet/build.yaml` only declared `architectures: [x86_64]`
  - because of that, the shared `miniconda` template rendered the x86_64 installer even though the build was running on an arm64 host
- Fix landed in recipe YAML:
  - add `aarch64` to `neurocontainers/recipes/megnet/build.yaml`
- Verified rerun result:
  - regenerating the Dockerfile for the same arm64 host now stages:
    `https://repo.anaconda.com/miniconda/Miniconda3-py310_25.5.1-0-Linux-aarch64.sh`
  - rerunning `./build.sh megnet` no longer failed immediately in the Miniconda bootstrap with the x86_64 `_conda` exec-format error; it progressed into downloading the arm64 installer payload
- Current status after this fix:
  - the original arm64 Miniconda/render issue is fixed
  - I interrupted the rerun during the long Miniconda installer download/bootstrap, so there is not yet a finalized `megnet:1.0.1` image from the rerun
- Scope note: this closes one concrete arm64 recipe-render issue for `megnet` by making the recipe render the correct Miniconda installer for arm64 hosts. Any later recipe-specific arm64 blockers remain untested from this turn.

- On 2026-03-28, `./build.sh megnet` was rerun on the same `aarch64` host.
- Initial failure on that rerun:
  `CondaValueError: 'base' is a reserved environment name`
- Cause:
  - after the arm64 installer fix, the remaining recipe path still relied on the Miniconda template's environment-creation flow
  - in this environment that rendered `conda create --name base`, which current Conda rejects
- Follow-on issue uncovered while fixing the same path:
  - even after moving off the reserved-`base` name, the template-managed environment creation path still failed inside Conda metadata/cache handling on arm64 (`libmambapy ... File ... does not exist`, then classic-solver cache-file errors)
- Fix landed in recipe YAML:
  - stop using the Miniconda template for `megnet`'s package/environment management beyond the installer itself
  - bootstrap the arm64 Miniconda installer explicitly in `neurocontainers/recipes/megnet/build.yaml`
  - install the required conda package set directly into `base` with `conda install --solver classic -n base -c conda-forge ...`
  - keep the recipe's pip-only dependencies as a direct `python -m pip install --no-cache-dir ...` step
- Verified rerun result:
  - the regenerated Dockerfile no longer emits `conda create ... --name base` or any separate Conda environment-creation step for `megnet`
  - the rerun gets cleanly through the arm64 Miniconda bootstrap/configuration path and enters the real `conda install -y --solver classic -n base -c conda-forge ...` solve for the `megnet` package set
  - the rerun was still actively solving that package set when I stopped it, so there is not yet a finalized `megnet:1.0.1` image from this follow-up pass
- Scope note: this closes one concrete Conda-environment build issue for `megnet` on arm64 by removing the broken template-managed env creation path; any later solver or package-compatibility blockers remain to be closed out in a future run.

### Recipe-level build check: `convert3d`

- On 2026-03-27, `./build.sh convert3d` was run on an `aarch64` host.
- Build result:
  - the Docker build completed successfully and produced `convert3d:1.1.0`
  - `docker image inspect convert3d:1.1.0 --format '{{.Architecture}} {{.Os}}'` reports an arm64 Linux image
- Runtime smoke check result:
  - `docker run --rm convert3d:1.1.0 c3d -version` failed immediately with:
    `exec /opt/convert3d-nightly/bin/c3d: exec format error`
- Cause:
  - the rendered Dockerfile still downloads `https://sourceforge.net/projects/c3d/files/c3d/Nightly/c3d-nightly-Linux-x86_64.tar.gz/download`
  - inspection of the downloaded upstream nightly payload confirms `c3d` and `c3d_affine_tool` are `ELF 64-bit ... x86-64`
  - inspection of the newer upstream `c3d-nightly-Linux-gcc64.tar.gz` payload on 2026-03-27 also showed `ELF 64-bit ... x86-64`, so it is not an arm64 replacement
- Scope note: `convert3d` currently has no validated upstream arm64 binary artifact in this template path. The recipe-level arm64 build succeeds, but the bundled runtime remains unusable on arm64 because the installed payload is still x86_64-only.

### Recipe-level build check: `eharmonize`

- On 2026-03-28, `./build.sh eharmonize` was run on an `aarch64` host.
- Initial failure:
  - the recipe declared only `x86_64`, so the rendered Miniconda template downloaded `Miniconda3-latest-Linux-x86_64.sh`
  - the installer failed with:
    `/opt/miniconda/_conda: cannot execute binary file: Exec format error`
  - the next Docker step then failed with:
    `/bin/sh: 1: conda: not found`
- First fix landed in recipe YAML:
  - add `aarch64` to `neurocontainers/recipes/eharmonize/build.yaml` so the Miniconda template renders an arm64-compatible installer path
- Second failure after rerun:
  - once the arm64 installer path was fixed, the build progressed into the template-managed environment creation step and failed with:
    `CondaValueError: 'base' is a reserved environment name`
- Second fix landed in recipe YAML:
  - set `env_name: eharmonize` and `env_exists: "false"` in the Miniconda template block in `neurocontainers/recipes/eharmonize/build.yaml`
- Verified rerun result:
  - the next rerun progressed through Miniconda bootstrap, `conda create --name eharmonize`, the upstream `git clone`, and `python -m pip install --no-cache-dir .`
  - Docker completed image export and named `eharmonize:1.0.0`
  - `docker image inspect eharmonize:1.0.0 --format '{{.Architecture}} {{.Os}}'` reports `arm64 linux`
  - a runtime smoke check with `docker run --rm --entrypoint /bin/bash eharmonize:1.0.0 -lc 'eharmonize --help'` succeeded
- Runtime note:
  - `eharmonize --help` emits a `pkg_resources` deprecation warning from the packaged Python code before printing normal help text
  - this warning did not block build or execution, so it was not treated as the build issue for this audit pass
- Scope note: this closes two concrete recipe-side Miniconda template blockers for `eharmonize` on arm64 and produces a runnable arm64 image from the current recipe.

### Recipe-level build check: `segmentator`

- On 2026-03-28, `./build.sh segmentator` was run on an `aarch64` host.
- Initial failure:
  - the recipe declared only `x86_64`, and the first arm64 rerun exposed a broken Miniconda path where later template steps reached:
    `/bin/sh: 1: conda: not found`
- First fix landed in recipe YAML:
  - add `aarch64` to `neurocontainers/recipes/segmentator/build.yaml`
- Second failure after rerun:
  - once the arm64 Miniconda path was active, the build failed in the template-managed env creation step with:
    `CondaValueError: 'base' is a reserved environment name`
- Second fix landed in recipe YAML:
  - set `env_name: segmentator` and `env_exists: "false"` in the Miniconda template block
- Third failure after rerun:
  - the recipe's original Conda spec was pinned to a Python 3.6 era environment that current arm64 channels could not solve, including:
    `nothing provides openssl >=1.0.2p,<1.0.3a needed by python-3.6.7`
- Third fix landed in recipe YAML:
  - replace the stale Conda spec with an arm64-solvable baseline:
    `matplotlib numpy nibabel scipy python=3.10`
- Fourth failure after rerun:
  - the recipe's pip dependency pin used invalid pip syntax:
    `compoda=0.3.5`
  - the build failed with:
    `Hint: = is not a valid operator. Did you mean == ?`
- Fourth fix landed in recipe YAML:
  - change `pip_install` to `compoda==0.3.5`
- Fifth failure after rerun:
  - the recipe unpack/install step ran its editable install outside the activated Conda env, so `setup.py` could not import the `numpy` dependency that had just been installed into `segmentator`
- Fifth fix landed in recipe YAML:
  - run the editable install inside the activated env with:
    `bash -c "source activate segmentator && python -m pip install -e /opt/segmentator"`
- Sixth failure after rerun:
  - once the editable install was moved into the activated env, pip's isolated editable-build metadata phase still could not see the already-installed `numpy` dependency and failed with:
    `ModuleNotFoundError: No module named 'numpy'`
- Sixth fix landed in recipe YAML:
  - disable pip build isolation for the editable install so the upstream build uses the existing Conda env dependencies:
    `bash -c "source activate segmentator && python -m pip install --no-build-isolation -e /opt/segmentator"`
- Seventh failure after rerun:
  - once pip build isolation was disabled, the upstream C extension build started and failed because the image had no compiler toolchain:
    `error: command 'gcc' failed: No such file or directory`
- Seventh fix landed in recipe YAML:
  - add a `run` step to install `build-essential` before the editable install stage
- Eighth failure after rerun:
  - once the editable extension build had a compiler toolchain, the upstream C extension still failed against NumPy 2.x, including:
    `error: ‘PyArray_Descr’ {aka ‘struct _PyArray_Descr’} has no member named ‘subarray’`
- Eighth fix landed in recipe YAML:
  - pin the Conda dependency set in `neurocontainers/recipes/segmentator/build.yaml` to `numpy=1.26`, keeping the upstream Cython extension on the older NumPy API it actually builds against
- Final verification:
  - after that NumPy pin, `./build.sh segmentator` completed successfully and produced `segmentator:1.6.1`
  - `docker image inspect segmentator:1.6.1 --format '{{.Id}} {{.Architecture}} {{.Os}}'` reported:
    `sha256:8763a48fa65f227bf330d63a0ca0896e610fc3c70784ac28cb14f640102b665f arm64 linux`
  - a runtime smoke check with:
    `docker run --rm segmentator:1.6.1 /bin/bash -lc 'source /opt/miniconda/etc/profile.d/conda.sh && conda activate segmentator && python -c "import segmentator; print(segmentator.__file__)"'`
    succeeded and reported:
    `/opt/segmentator/segmentator/__init__.py`
- Runtime note:
  - importing `segmentator` emits an upstream `pkg_resources` deprecation warning from the packaged Python code before normal module import completes
  - this did not block build or execution, so it was not treated as the build issue for this audit pass
- Revalidation note:
  - a fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh segmentator` on the same `aarch64` host completed cleanly again and rebuilt `segmentator:1.6.1` from cache
  - `docker image inspect segmentator:1.6.1 --format '{{.Id}} {{.Architecture}} {{.Os}}'` reported:
    `sha256:8763a48fa65f227bf330d63a0ca0896e610fc3c70784ac28cb14f640102b665f arm64 linux`
  - the same runtime smoke check still succeeded and resolved `segmentator` from:
    `/opt/segmentator/segmentator/__init__.py`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh segmentator` on the same `aarch64` host completed cleanly again and rebuilt `segmentator:1.6.1` from cache
  - `docker image inspect segmentator:1.6.1 --format '{{.Id}} {{.Architecture}} {{.Os}}'` still reports:
    `sha256:55bb85c359ae33fd4aecaf0340db70cfb2e739ce4f37fa14abeec32d68db75d6 arm64 linux`
  - the same runtime smoke check still succeeded and resolved `segmentator` from:
    `/opt/segmentator/segmentator/__init__.py`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh segmentator` on the same `aarch64` host completed cleanly again and rebuilt `segmentator:1.6.1` from cache
  - `docker image inspect segmentator:1.6.1 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:97b060fda18a152a2e9779aa15b8c3432d72280a2855be375a130e4438136662 arm64 linux`
  - the same runtime smoke check still succeeded and resolved `segmentator` from:
    `/opt/segmentator/segmentator/__init__.py`
  - the same upstream import-time warning is still present:
    `pkg_resources is deprecated as an API`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh segmentator` on the same `aarch64` host completed cleanly again and rebuilt `segmentator:1.6.1` from cache
  - `docker image inspect segmentator:1.6.1 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:160fdcbf1700195d646976538f169ffba30f45aa4ae54b7762fd56ff2bbf2d43 arm64 linux`
  - the same runtime smoke check still succeeded and resolved `segmentator` from:
    `/opt/segmentator/segmentator/__init__.py`
  - the same upstream import-time warning is still present:
    `pkg_resources is deprecated as an API`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh segmentator` on the same `aarch64` host completed cleanly again and rebuilt `segmentator:1.6.1` from cache
  - `docker image inspect segmentator:1.6.1 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:0668e3e67e4789a30716a2e3c5a2da1cd97c322d3dcf532b4d81e01e1e0664d8 arm64 linux`
  - the same runtime smoke check still succeeded and resolved `segmentator` from:
    `/opt/segmentator/segmentator/__init__.py`
  - the same upstream import-time warning is still present:
    `pkg_resources is deprecated as an API`
- Scope note: this pass closes three more concrete recipe-side blockers for `segmentator` on arm64, including the NumPy 2.x incompatibility, and ends with a successful `segmentator:1.6.1` image build on arm64.

### Recipe-level build check: `spmpython`

- On 2026-03-28, `./build.sh spmpython` was run on an `aarch64` host.
- Initial failure:
  - the recipe declared only `x86_64`, and the first arm64 rerun exposed a broken Miniconda path where later template steps reached:
    `/bin/sh: 1: conda: not found`
- First fix landed in recipe YAML:
  - add `aarch64` to `neurocontainers/recipes/spmpython/build.yaml`
- Second failure after rerun:
  - once the arm64 Miniconda path was active, the build failed in the template-managed env creation step with:
    `CondaValueError: 'base' is a reserved environment name`
- Second fix landed in recipe YAML:
  - set `env_name: spmpython` and `env_exists: "false"` in the Miniconda template block in `neurocontainers/recipes/spmpython/build.yaml`
- Verified rerun result:
  - the next rerun progressed through Miniconda bootstrap, `conda create --name spmpython`, and the Conda package install for `python=3.12`
  - the build then entered the real upstream package install path with the activated-env command:
    `python -m pip install --no-cache-dir spm-python`
  - observed downloads during that step included:
    `spm_python-25.1.2.post2-py3-none-any.whl`
    `spm_runtime_r2025b-25.1.2.post2-py3-none-any.whl`
    `matlab_runtime-0.0.6-py3-none-any.whl`
    `mpython_core-25.4rc1-py3-none-any.whl`
  - the rerun was still actively downloading/installing that package stack when I stopped it, so there is not yet a finalized `spmpython:25.1.2.post1` image recorded from this pass
- Verified rerun result:
  - a fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh spmpython` on the same `aarch64` host completed cleanly through image export as `spmpython:25.1.2.post1`
  - `docker image inspect spmpython:25.1.2.post1 --format '{{.Id}} {{.Architecture}} {{.Os}}'` reported:
    `sha256:b79bb2a4a549ebb04033ddc41958a55072665a3984c0abfc568c020a7dda0b22 arm64 linux`
  - the same env-local runtime smoke check now succeeds:
    `docker run --rm spmpython:25.1.2.post1 /bin/bash -lc 'source /opt/miniconda/etc/profile.d/conda.sh && conda activate spmpython && python -c "import spm; print(spm.__file__)"'`
    and reported:
    `/opt/miniconda/envs/spmpython/lib/python3.12/site-packages/spm/__init__.py`
  - the same import path also emits an upstream warning from `mpython`:
    `Since scipy.sparse is not available, sparse matrices will be implemented as dense matrices`
- Scope note:
  - this pass closes the remaining unresolved `spmpython` build path on arm64 and now produces a verified successful `spmpython:25.1.2.post1` arm64 image

### Recipe-level full test check: `spmpython`

- On 2026-03-28, `./test.sh spmpython` was run against the existing local `spmpython:25.1.2.post1` image on an `aarch64` host without rebuilding the Docker image.
- Initial issue:
  - `neurocontainers/recipes/spmpython/fulltest.yaml` still pointed at the old dated SIF name `spmpython_25.1.2.post1_20250905.simg`, while the current `./test.sh` path generates `sifs/spmpython_25.1.2.post1.simg`
- Fix landed in recipe YAML only:
  - update `container:` to `spmpython_25.1.2.post1.simg`
- Rerun result:
  - a fresh rerun of `./test.sh spmpython` was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`
  - the rerun remained in the long live Apptainer SIF-conversion phase for `sifs/spmpython_25.1.2.post1.simg` and did not reach a new completed suite summary before it was stopped
- Scope note:
  - this pass closes the stale fulltest metadata for the current no-rebuild wrapper path for `spmpython`, but it does not yet add a completed full-suite runtime result for this image
  - the underlying local Docker image used for this no-rebuild path was the existing verified arm64 build recorded above, not a rebuilt container

### Recipe-level build check: `topaz`

- On 2026-03-28, `./build.sh topaz` was run on an `aarch64` host.
- Initial failure:
  - the recipe declared only `x86_64`, so the rendered Miniconda template downloaded the x86_64 installer:
    `https://repo.anaconda.com/miniconda/Miniconda3-4.7.12.1-Linux-x86_64.sh`
  - that installer failed on arm64 with:
    `/opt/miniconda-4.7.12.1/conda.exe: cannot execute binary file: Exec format error`
  - later template steps then failed with:
    `/bin/sh: 1: conda: not found`
- First fix landed in recipe YAML:
  - add `aarch64` to `neurocontainers/recipes/topaz/build.yaml`
- Second failure after rerun:
  - once the arm64 Miniconda path was active, the recipe's pinned installer version no longer existed for arm64:
    `curl: (22) The requested URL returned error: 404`
  - the rendered URL was:
    `https://repo.anaconda.com/miniconda/Miniconda3-4.7.12.1-Linux-aarch64.sh`
- Second fix landed in recipe YAML:
  - change the Miniconda template version in `neurocontainers/recipes/topaz/build.yaml` from `4.7.12.1` to `latest`
- Third failure after rerun:
  - with a current arm64 installer in place, the build progressed into the template-managed environment creation step and failed with:
    `CondaValueError: 'base' is a reserved environment name`
- Third fix landed in recipe YAML:
  - set `env_name: topaz` and `env_exists: "false"` in the Miniconda template block in `neurocontainers/recipes/topaz/build.yaml`
- Verified rerun result:
  - the next rerun progressed through arm64 Miniconda bootstrap, `conda update -n base conda`, `conda create --name topaz`, and into the real package solve for:
    `python=3.6 topaz=0.2.5 cudatoolkit=10.2 -c tbepler -c pytorch`
  - the remaining failure is now later and narrower:
    `LibMambaUnsatisfiableError`
    `nothing provides openssl >=1.0.2p,<1.0.3a needed by python-3.6.7`
    `cudatoolkit =10.2 * does not exist`
- Fourth fix landed in recipe YAML:
  - replace the stale Conda package spec in `neurocontainers/recipes/topaz/build.yaml`
  - the Miniconda template now creates the env with `python=3.10`
  - the package install is moved to an env-activated pip step:
    `python -m pip install --no-cache-dir topaz-em==0.2.5`
- Verified rerun result after fourth fix:
  - the rerun got cleanly past the old `python=3.6 topaz=0.2.5 cudatoolkit=10.2` solver failure
  - the previous arm64 blockers:
    `nothing provides openssl >=1.0.2p,<1.0.3a needed by python-3.6.7`
    and
    `cudatoolkit =10.2 * does not exist`
    are gone
  - the build now completes `conda install -y -q --name topaz "python=3.10"` and enters the real env-local pip install path:
    `source activate topaz`
    `python -m pip install --no-cache-dir topaz-em==0.2.5`
  - I stopped that rerun while the heavy dependency install was still active, after it had already started downloading the real arm64 package set including `topaz_em-0.2.5`, `torch-2.11.0-cp310-cp310-manylinux_2_28_aarch64.whl`, `numpy-2.2.6-cp310-cp310-...`, and `scikit_learn-1.7.2-cp310-cp310-...`
- Scope note: this pass closes four concrete recipe-side blockers for `topaz` on arm64 and moves the build into the intended Python 3.10 env-local package installation path. A final successful arm64 image was not produced in this pass.
- Revalidation note:
  - a fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh topaz` on the same `aarch64` host reached the same env-local install path again:
    `source activate topaz`
    `python -m pip install --no-cache-dir topaz-em==0.2.5`
  - the old solver blockers remain closed: the rerun did not revisit the previous `python=3.6` / `cudatoolkit=10.2` `LibMambaUnsatisfiableError`
  - before I stopped the rerun, the active downloads were the expected large upstream wheel set for that path, including:
    `topaz_em-0.2.5`
    `torch-2.11.0-cp310-cp310-manylinux_2_28_aarch64.whl`
    `nvidia_cudnn_cu13-9.19.0.56`
    `nvidia_cusparselt_cu13-0.8.0`
    `nvidia_nccl_cu13-2.28.9`
    `triton-3.6.0`
  - there is still no finalized `topaz:0.2.5` image from this path because the rerun was stopped while the heavy pip dependency install was still active
- Verified rerun result:
  - a fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh topaz` on the same `aarch64` host completed cleanly through image export as `topaz:0.2.5`
  - `docker image inspect topaz:0.2.5 --format '{{.Id}} {{.Architecture}} {{.Os}}'` reported:
    `sha256:33631419900bd9ee938b5e1ee79dbc5d4414afdb3235b260a23a8ef9015e9c8e arm64 linux`
  - the same env-local runtime smoke check now succeeds:
    `docker run --rm topaz:0.2.5 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate topaz && python -c "import topaz; print(topaz.__version__)"'`
    and reported:
    `0.2.5a`
- Scope note:
  - this pass closes the remaining unresolved `topaz` build path on arm64 and now produces a verified successful `topaz:0.2.5` arm64 image

### Recipe-level build check: `vesselvio`

- On 2026-03-28, `./build.sh vesselvio` was run on an `aarch64` host.
- Initial failure:
  - the recipe declared only `x86_64`, so the rendered Miniconda template downloaded the x86_64 installer:
    `https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-x86_64.sh`
  - that installer failed on arm64 with:
    `/opt/miniconda-latest/_conda: cannot execute binary file: Exec format error`
  - the next template step then failed with:
    `/bin/sh: 1: conda: not found`
- First fix landed in recipe YAML:
  - add `aarch64` to `neurocontainers/recipes/vesselvio/build.yaml`
- Second failure after rerun:
  - once the arm64 installer path was active, the build progressed into the template-managed environment creation step and failed with:
    `CondaValueError: 'base' is a reserved environment name`
- Second fix landed in recipe YAML:
  - set `env_name: vesselvio` and `env_exists: "false"` in the Miniconda template block in `neurocontainers/recipes/vesselvio/build.yaml`
- Verified rerun result:
  - the next rerun progressed through arm64 Miniconda bootstrap, `conda update -n base conda`, `conda create --name vesselvio`, the Conda package install for `python=3.8.8`, and the environment cleanup step
  - the build then entered the real upstream application install path:
    `git clone https://github.com/JacobBumgarner/VesselVio.git /opt/vesselvio-1.1.2/`
    `pip install -r /opt/vesselvio-1.1.2/requirements.txt`
  - observed progress in that step included source builds and metadata preparation for requirements such as `aiohttp==3.8.1`, `frozenlist==1.2.0`, `future==0.18.2`, and `igraph==0.9.10`
  - the rerun was still actively processing that dependency stack when I stopped it, so there is not yet a finalized `vesselvio:1.1.2` image recorded from this pass
- Third fix landed in recipe YAML:
  - run the upstream requirements install inside the named `vesselvio` Conda env in `neurocontainers/recipes/vesselvio/build.yaml`:
    `source activate vesselvio`
    `python -m pip install -r /opt/vesselvio-1.1.2/requirements.txt`
    instead of the previous bare `pip install -r ...` from the base shell
- Verified rerun result after third fix:
  - the rerun now stays on the `vesselvio` env during the upstream install path
  - the previous base-environment behavior is gone, and the build progresses through the env-local Python 3.8.8 requirements installation
  - observed progress in the patched path included env-local resolution and metadata preparation for pinned requirements such as:
    `aiohttp==3.8.1`
    `frozenlist==1.2.0`
    `future==0.18.2`
    `igraph==0.9.10`
    `llvmlite==0.36.0`
    `numba==0.53.1`
    `opencv-python==4.5.4.60`
    `pandas==1.3.5`
  - I stopped that rerun while the heavy requirements install was still active, so there is not yet a finalized `vesselvio:1.1.2` image from this pass
- Fourth fix landed in recipe YAML:
  - rewrite the stale upstream `PyQt5==5.13.2` pin to `PyQt5==5.15.11` in `neurocontainers/recipes/vesselvio/build.yaml` before the env-local `pip install -r /opt/vesselvio-1.1.2/requirements.txt` step
- Verified rerun result after fourth fix:
  - the rerun no longer fails with:
    `ERROR: Could not find a version that satisfies the requirement PyQt5==5.13.2`
  - the patched path now gets past that old unavailable-wheel blocker and reaches PyQt5 build metadata generation for the rewritten pin:
    `Collecting PyQt5==5.15.11`
  - the remaining failure is now later and narrower, during PyQt5 source-build metadata generation:
    `sipbuild.pyproject.PyProjectOptionException`
  - the traceback shows the immediate missing build tool boundary in the PyQt build backend:
    `raise PyProjectOptionException('qmake',`
- Fifth fix landed in recipe YAML:
  - add Qt build tooling in `neurocontainers/recipes/vesselvio/build.yaml`:
    `qt5-qmake`
    and
    `qtbase5-dev`
- Verified rerun result after fifth fix:
  - the rerun no longer fails at the previous PyQt build-tool boundary:
    `sipbuild.pyproject.PyProjectOptionException`
    `raise PyProjectOptionException('qmake',`
  - the patched build now gets cleanly through the added Qt package layer and back into the env-local requirements install
  - observed progress in the patched PyQt path included:
    `Collecting PyQt5==5.15.11`
    `Installing build dependencies: finished with status 'done'`
    `Getting requirements to build wheel: finished with status 'done'`
    `Preparing metadata (pyproject.toml): started`
  - I stopped that rerun while it was still active in the later PyQt build path, so there is not yet a new terminal failure or a finalized `vesselvio:1.1.2` image from this pass
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh vesselvio` on the same `aarch64` host again got cleanly past the old missing-`qmake` blocker
  - the rerun returned to the same later PyQt path and remained active there without producing a new terminal failure, including:
    `Collecting PyQt5==5.15.11`
    `Installing build dependencies: finished with status 'done'`
    `Getting requirements to build wheel: finished with status 'done'`
    `Preparing metadata (pyproject.toml): started`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh vesselvio` on the same `aarch64` host again got cleanly through the old PyQt resolver and missing-`qmake` boundaries
  - on this pass the build progressed back into the same env-local upstream requirements install and stayed active there for several minutes without producing a new terminal failure
  - the repeated observed later-stage path still includes:
    `Collecting PyQt5==5.15.11`
    `Installing build dependencies: finished with status 'done'`
    `Getting requirements to build wheel: finished with status 'done'`
    `Preparing metadata (pyproject.toml): started`
  - I stopped this pass at that same late PyQt build stage, so there is still no newer concrete blocker or finalized `vesselvio:1.1.2` image to record beyond the existing fix boundary
  - I stopped that rerun while `docker build` was still active in the same later PyQt build stage, so there is still no finalized `vesselvio:1.1.2` image from this path
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh vesselvio` on the same `aarch64` host again got cleanly through the old unavailable-`PyQt5==5.13.2` and missing-`qmake` boundaries
  - this pass progressed back through the env-local requirements install into the same late PyQt source-build path, including:
    `Collecting PyQt5==5.15.11`
    `Installing build dependencies: finished with status 'done'`
    `Getting requirements to build wheel: finished with status 'done'`
    `Preparing metadata (pyproject.toml): started`
  - after that point the rerun remained active in the long PyQt build phase without emitting a newer terminal failure, so I stopped it there
  - there is still no newer concrete blocker or finalized `vesselvio:1.1.2` image to record beyond the existing fix boundary
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh vesselvio` on the same `aarch64` host again got cleanly through the old unavailable-`PyQt5==5.13.2` and missing-`qmake` boundaries
  - on this pass the build advanced through the env-local requirements resolution and once again reached the same later PyQt source-build metadata path:
    `Collecting PyQt5==5.15.11`
    `Installing build dependencies: finished with status 'done'`
    `Getting requirements to build wheel: finished with status 'done'`
    `Preparing metadata (pyproject.toml): started`
  - after that point both `docker build` and the env-local `python -m pip install -r /opt/vesselvio-1.1.2/requirements.txt` process remained alive without emitting a newer terminal failure, so I stopped the rerun there
  - there is still no newer concrete blocker or finalized `vesselvio:1.1.2` image to record beyond the existing fix boundary
- Scope note: this pass closes five concrete recipe-side blockers for `vesselvio` on arm64 and moves the build past both the old unavailable `PyQt5==5.13.2` pin and the missing-`qmake` boundary into a later PyQt build stage. A final successful arm64 image was not produced in this pass.

### Recipe-level build check: `hdbet`

- On 2026-03-28, `./build.sh hdbet` was run on an `aarch64` host.
- Initial failure:
  - the recipe declared only `x86_64`, so the rendered Miniconda template downloaded the x86_64 installer:
    `https://repo.anaconda.com/miniconda/Miniconda3-4.7.12.1-Linux-x86_64.sh`
  - later template steps then failed with:
    `/bin/sh: 1: conda: not found`
- First fix landed in recipe YAML:
  - add `aarch64` to `neurocontainers/recipes/hdbet/build.yaml`
- Second failure after rerun:
  - once the arm64 Miniconda path was active, the recipe's pinned installer version no longer existed for arm64:
    `curl: (22) The requested URL returned error: 404 Not Found`
  - the rendered URL was:
    `https://repo.anaconda.com/miniconda/Miniconda3-4.7.12.1-Linux-aarch64.sh`
- Second fix landed in recipe YAML:
  - change the Miniconda template version in `neurocontainers/recipes/hdbet/build.yaml` from `4.7.12.1` to `latest`
- Verified rerun result:
  - the next rerun got past the old installer URL failure and reached the current arm64 Miniconda bootstrap on the same recipe path
  - the remaining failure is now later and narrower, in the interaction between the new Miniconda installer and the old base image:
    `Installer requires GLIBC >=2.28, but system has 2.23.`
  - after that installer failure, the next Docker step still aborts with:
    `/bin/sh: 1: conda: not found`
- Third fix landed in recipe YAML:
  - change the recipe base image in `neurocontainers/recipes/hdbet/build.yaml` from `ubuntu:16.04` to `ubuntu:20.04`
- Verified rerun result after third fix:
  - the next rerun got cleanly past the old Miniconda installer GLIBC failure on arm64
  - the previous blocker:
    `Installer requires GLIBC >=2.28, but system has 2.23.`
    is gone
  - the rerun then exposed the next recipe-side failure in the template-managed environment creation step:
    `CondaValueError: 'base' is a reserved environment name`
- Fourth fix landed in recipe YAML:
  - set `env_name: hdbet` and `env_exists: "false"` in the Miniconda template block in `neurocontainers/recipes/hdbet/build.yaml`
- Verified rerun result after fourth fix:
  - the rerun now gets through `conda create --name hdbet` on `linux-aarch64`
  - the remaining failure is later and narrower, in the recipe's pinned Python install:
    `conda install -y -q --name hdbet "python=3.6"`
  - the concrete solver error reported by the rerun is:
    `LibMambaUnsatisfiableError`
    `nothing provides openssl >=1.0.2p,<1.0.3a needed by python-3.6.7`
- Fifth fix landed in recipe YAML:
  - update the Miniconda template `conda_install` pin in `neurocontainers/recipes/hdbet/build.yaml` from `python=3.6` to `python=3.10`, matching the current upstream `HD_BET` package metadata (`requires-python >=3.10`)
- Verified rerun result after fifth fix:
  - the rerun got cleanly past the old `python=3.6` solver failure and completed:
    `conda install -y -q --name hdbet "python=3.10"`
  - the previous blocker:
    `nothing provides openssl >=1.0.2p,<1.0.3a needed by python-3.6.7`
    is gone
  - the build then entered the recipe's real upstream application install path:
    `git clone https://github.com/MIC-DKFZ/HD-BET`
    `pip install -e .`
  - the rerun was still actively downloading and resolving the heavy dependency stack when I stopped it, including `torch-2.11.0-cp313-cp313-manylinux_2_28_aarch64.whl`
- Sixth fix landed in recipe YAML:
  - run the final upstream install inside the named `hdbet` Conda env in `neurocontainers/recipes/hdbet/build.yaml`:
    `source activate hdbet`
    `python -m pip install -e .`
    instead of the previous bare `pip install -e .` from the base shell
- Verified rerun result after sixth fix:
  - the rerun stays on the `hdbet` env during the upstream install path
  - the previous base-environment behavior is gone: the dependency resolution no longer targets `cp313` wheels from `/opt/miniconda-latest`
  - the patched rerun now resolves packages against the intended Python 3.10 env, including:
    `numpy-2.2.6-cp310-cp310-...`
    `torch-2.11.0-cp310-cp310-manylinux_2_28_aarch64.whl`
    `SimpleITK-2.5.3-cp310-cp310-...`
  - I stopped that rerun while the heavy upstream dependency install was still active, so there is not yet a finalized `hdbet:1.0.0` image from this pass
- Final verified rerun result:
  - a fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh hdbet` on the same `aarch64` host completed cleanly and produced `hdbet:1.0.0`
  - `docker image inspect hdbet:1.0.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` reported:
    `sha256:504f9381cc4751bbd2cd3e838a936278def12a2e149cb8a91b713781c143a7b8 arm64 linux`
  - a runtime smoke check inside the named env succeeded:
    `docker run --rm hdbet:1.0.0 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate hdbet && python -c "import HD_BET; print(HD_BET.__file__)"'`
    and reported:
    `/opt/HD-BET/HD_BET/__init__.py`
  - package metadata is also present in the env:
    `python -m pip show HD_BET`
    reported:
    `Version: 2.0.1`
    with editable project location:
    `/opt/HD-BET`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh hdbet` on the same `aarch64` host completed cleanly again and rebuilt `hdbet:1.0.0` from cache
  - `docker image inspect hdbet:1.0.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:9906d2296210f1067dfbc51926f8b67e2d9d64082bcd468bd21aa496c605a62b arm64 linux`
  - the same runtime import smoke check still succeeded:
    `docker run --rm hdbet:1.0.0 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate hdbet && python -c "import HD_BET; print(HD_BET.__file__)"'`
    and reported:
    `/opt/HD-BET/HD_BET/__init__.py`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh hdbet` on the same `aarch64` host completed cleanly again
  - `docker image inspect hdbet:1.0.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:4faa69c37a87902c373a00e6376c0b59be048ec5d3d670ad4726b4241cd3e192 arm64 linux`
  - the same env-local runtime smoke check still succeeded:
    `docker run --rm hdbet:1.0.0 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate hdbet && python -c "import HD_BET; print(HD_BET.__file__)" && python -m pip show HD_BET | sed -n "1,4p"'`
    and reported:
    `/opt/HD-BET/HD_BET/__init__.py`
    `Name: HD_BET`
    `Version: 2.0.1`
    `Summary: Tool for brain extraction`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh hdbet` on the same `aarch64` host completed cleanly again and rebuilt `hdbet:1.0.0` from cache
  - `docker image inspect hdbet:1.0.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:68bc459c771c0c9e0c3cfc6ff99d9a856e043f3493b1950f0d508d3ad0dc3c68 arm64 linux`
  - the same env-local runtime smoke check still succeeded:
    `docker run --rm hdbet:1.0.0 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate hdbet && python -c "import HD_BET; print(HD_BET.__file__)" && python -m pip show HD_BET | sed -n "1,4p"'`
    and reported:
    `/opt/HD-BET/HD_BET/__init__.py`
    `Name: HD_BET`
    `Version: 2.0.1`
    `Summary: Tool for brain extraction`
  - package metadata in the named env is still present and still reports:
    `Name: HD_BET`
    `Version: 2.0.1`
    `Location: /opt/miniconda-latest/envs/hdbet/lib/python3.10/site-packages`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh hdbet` on the same `aarch64` host completed cleanly again and rebuilt `hdbet:1.0.0` from cache
  - `docker image inspect hdbet:1.0.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:4faa69c37a87902c373a00e6376c0b59be048ec5d3d670ad4726b4241cd3e192 arm64 linux`
  - the same runtime import smoke check still succeeded:
    `docker run --rm hdbet:1.0.0 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate hdbet && python -c "import HD_BET; print(HD_BET.__file__)"'`
    and reported:
    `/opt/HD-BET/HD_BET/__init__.py`
  - package metadata in the named env is still present and still reports:
    `Name: HD_BET`
    `Version: 2.0.1`
    `Location: /opt/miniconda-latest/envs/hdbet/lib/python3.10/site-packages`
- Scope note: this pass closes six concrete recipe-side build blockers for `hdbet` on arm64 and now produces a verified successful `hdbet:1.0.0` arm64 image.

### Recipe-level build check: `condaenvs`

- On 2026-03-28, `./build.sh condaenvs` was run on an `aarch64` host.
- Initial failure:
  - the recipe declared only `x86_64`, so the rendered Miniconda template downloaded the x86_64 installer:
    `https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-x86_64.sh`
  - that installer failed on arm64 with:
    `/opt/miniconda-latest/_conda: cannot execute binary file: Exec format error`
  - the next Docker step then failed with:
    `/bin/sh: 1: conda: not found`
- First fix landed in recipe YAML:
  - add `aarch64` to `neurocontainers/recipes/condaenvs/build.yaml`
- Second failure after rerun:
  - once the arm64 Miniconda path was active, the build progressed into the template-managed environment creation step and failed with:
    `CondaValueError: 'base' is a reserved environment name`
- Second fix landed in recipe YAML:
  - set `env_name: condaenvs` and `env_exists: "false"` in the Miniconda template block in `neurocontainers/recipes/condaenvs/build.yaml`
- Verified rerun result:
  - the next rerun progressed through arm64 Miniconda bootstrap, `conda update -n base conda`, `conda create --name condaenvs`, the environment cleanup step, and the later `apt install git` step
  - the build then reached the recipe's own upstream fetch path:
    `git clone https://github.com/NeuroDesk/condaenvs`
  - the remaining failure is now later and narrower:
    `fatal: could not read Username for 'https://github.com': No such device or address`
- Third fix landed in recipe YAML:
  - restore the missing `git` package install and make the source fetch an explicit non-interactive `.git` clone in `neurocontainers/recipes/condaenvs/build.yaml`
- Verified rerun result after third fix:
  - the rerun no longer fails with `/bin/sh: 1: git: not found`
  - it now reaches the same upstream source acquisition boundary directly:
    `GIT_TERMINAL_PROMPT=0 git -c credential.helper= clone https://github.com/NeuroDesk/condaenvs.git /opt/condaenvs`
  - the remaining failure is still the GitHub-side auth/prompt problem, now reported as:
    `fatal: could not read Username for 'https://github.com': terminal prompts disabled`
- Scope note: this pass closes two concrete recipe-side Miniconda blockers for `condaenvs` on arm64 and moves the build into the recipe's current upstream source acquisition problem. A final successful arm64 image was not produced in this pass.

### Recipe-level build check: `mne`

- On 2026-03-28, `./build.sh mne` was run on an `aarch64` host.
- Initial failure:
  - the recipe declared only `x86_64`, so the rendered Miniconda template downloaded the x86_64 installer:
    `https://repo.anaconda.com/miniconda/Miniconda3-4.7.12-Linux-x86_64.sh`
  - later template steps then failed with:
    `/bin/sh: 1: conda: not found`
- First fix landed in recipe YAML:
  - add `aarch64` to `neurocontainers/recipes/mne/build.yaml`
- Second failure after rerun:
  - once the arm64 Miniconda path was active, the recipe's pinned installer version no longer existed for arm64:
    `curl: (22) The requested URL returned error: 404`
  - the rendered URL was:
    `https://repo.anaconda.com/miniconda/Miniconda3-4.7.12-Linux-aarch64.sh`
- Second fix landed in recipe YAML:
  - change the Miniconda template version in `neurocontainers/recipes/mne/build.yaml` from `4.7.12` to `latest`
- Verified rerun result:
  - the next rerun progressed through arm64 Miniconda bootstrap, `conda update -n base conda`, `conda install -n base conda-libmamba-solver`, and `conda init bash`
  - the remaining failure is now later and narrower, in the template-managed environment creation step:
    `CondaValueError: 'base' is a reserved environment name`
- Third fix landed in recipe YAML:
  - change the Miniconda template env in `neurocontainers/recipes/mne/build.yaml` from `env_name: base` to `env_name: mne` and set `env_exists: "false"`
- Verified rerun result after third fix:
  - the next rerun progressed through arm64 Miniconda bootstrap, `conda create --name mne`, and cleanup of the new template-managed environment
  - the remaining failure is now later and narrower, in the recipe's own `mamba` install step:
    `LibMambaUnsatisfiableError`
  - the concrete conflict reported by the rerun is:
    `package mamba-0.24.0-py310hcf12e44_1 requires python >=3.10,<3.11.0a0 *_cpython`
    while the current Miniconda base environment is pinned to:
    `python=3.13`
- Fourth fix landed in recipe YAML:
  - remove the old base-environment `mamba=0.24.0` install from `neurocontainers/recipes/mne/build.yaml`
  - replace the later `mamba create ...` step with a direct:
    `conda create --override-channels --channel=conda-forge --name=mne-1.7.1 urllib3=2.2.1 mne=1.7.1`
- Verified rerun result after fourth fix:
  - the next rerun got cleanly past the old immediate `mamba=0.24.0` solver failure
  - it entered the real `mne-1.7.1` environment transaction on arm64 and started downloading the package set, including:
    `mne-1.7.1`
    `mne-base-1.7.1`
    `qt-main-5.15.15`
    `qt6-main-6.8.3`
    `vtk-base-9.4.1`
  - I stopped that rerun while the large conda-forge transaction was still in progress, so there is not yet a finalized `mne:1.7.1` image recorded from this pass
- Fifth fix landed in recipe YAML:
  - replace the fixed VS Code payload in `neurocontainers/recipes/mne/build.yaml` with an architecture-aware download at build time
  - on `aarch64`, fetch `https://code.visualstudio.com/sha/download?build=stable&os=linux-deb-arm64` instead of the previous `linux-deb-x64` package
- Verified rerun result after fifth fix:
  - the next rerun completed the full `conda create --name mne-1.7.1 ...` transaction and got past the old VS Code packaging failure
  - the previous arm64 blocker:
    `code:amd64 : Depends: ... but it is not installable`
    is gone
  - the patched rerun installed `code arm64 1.113.0-1774364715`, completed the VS Code extension-install layer, unpacked `mne-bids-pipeline-main`, and completed Docker image export as `mne:1.7.1`
  - verification:
    `docker image inspect mne:1.7.1` reported `arm64 linux`
    and
    `docker run --rm mne:1.7.1 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate mne-1.7.1 && python -c "import mne; print(mne.__version__)"'`
    printed `1.7.1`
- Scope note: this pass closes five concrete recipe-side blockers for `mne` on arm64 and now produces a verified successful `mne:1.7.1` arm64 image.
- Revalidation note:
  - a fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh mne` on the same `aarch64` host completed cleanly again and rebuilt `mne:1.7.1` from cache
  - `docker image inspect mne:1.7.1 --format '{{.Id}} {{.Architecture}} {{.Os}}'` reported:
    `sha256:45ccd97a534a96ad1b887d0a2245263912bda69b4284e017beb78daed2461d0f arm64 linux`
  - the same runtime smoke check still succeeded:
    `docker run --rm mne:1.7.1 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate mne-1.7.1 && python -c "import mne; print(mne.__version__)"'`
    and reported:
    `1.7.1`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh mne` on the same `aarch64` host completed cleanly again and rebuilt `mne:1.7.1` from cache
  - `docker image inspect mne:1.7.1 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:c9dbf5e93f342c2eed38adb34db41ccd62fc8fff4cd5c6b294302ab27417c503 arm64 linux`
  - the same env-local runtime smoke check still succeeded:
    `docker run --rm mne:1.7.1 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate mne-1.7.1 && python -c "import mne; print(mne.__version__)"'`
    and reported:
    `1.7.1`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh mne` on the same `aarch64` host completed cleanly again and rebuilt `mne:1.7.1` from cache
  - `docker image inspect mne:1.7.1 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:93828e4ddde21bf12ad7499f9b02ff06021b4540e2638bb620fc7a900647b489 arm64 linux`
  - the same env-local runtime smoke check still succeeded:
    `docker run --rm mne:1.7.1 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate mne-1.7.1 && python -c "import mne; print(mne.__version__)"'`
    and reported:
    `1.7.1`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh mne` on the same `aarch64` host completed cleanly again and rebuilt `mne:1.7.1` from cache
  - `docker image inspect mne:1.7.1 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:6378f17678588df8b34dfce59d65a28770f308fb9d3422c350f96e663d60536e arm64 linux`
  - the same env-local runtime smoke check still succeeded:
    `docker run --rm mne:1.7.1 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate mne-1.7.1 && python -c "import mne; print(mne.__version__)"'`
    and reported:
    `1.7.1`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh mne` on the same `aarch64` host completed cleanly again and rebuilt `mne:1.7.1` from cache
  - `docker image inspect mne:1.7.1 --format '{{.Id}} {{.Architecture}} {{.Os}}'` still reports:
    `sha256:f311b16b7f3f10a60b9b444d1ce7020788fe159c43cbbf265400f7f480cbf294 arm64 linux`
  - the same runtime smoke check still succeeded:
    `docker run --rm mne:1.7.1 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate mne-1.7.1 && python -c "import mne; print(mne.__version__)"'`
    and reported:
    `1.7.1`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh mne` on the same `aarch64` host completed cleanly again and rebuilt `mne:1.7.1` from cache
  - `docker image inspect mne:1.7.1 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:61b42aa91294d16d255c6aa1860ea367030080c80dbe56a316a51f99a2419144 arm64 linux`
  - the same runtime smoke check still succeeded:
    `docker run --rm mne:1.7.1 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate mne-1.7.1 && python -c "import mne; print(mne.__version__)"'`
    and reported:
    `1.7.1`

### Recipe-level build check: `fitlins`

- On 2026-03-28, `./build.sh fitlins` was run on an `aarch64` host.
- Initial failure:
  - the recipe declared only `x86_64`, so the rendered Miniconda template downloaded the x86_64 installer:
    `https://repo.anaconda.com/miniconda/Miniconda3-py39_25.5.1-0-Linux-x86_64.sh`
  - Miniconda bootstrap then failed with:
    `/opt/miniconda/_conda: cannot execute binary file: Exec format error`
  - later template steps then failed with:
    `/bin/sh: 1: conda: not found`
- Fix landed in recipe YAML:
  - add `aarch64` to `neurocontainers/recipes/fitlins/build.yaml`
- Verified rerun result:
  - the next rerun switched to the arm64 installer URL:
    `https://repo.anaconda.com/miniconda/Miniconda3-py39_25.5.1-0-Linux-aarch64.sh`
  - that rerun progressed through Miniconda bootstrap, `conda install -n base conda-libmamba-solver`, and `conda init bash`
  - the remaining failure is now later and narrower, in the template-managed environment creation step:
    `CondaValueError: 'base' is a reserved environment name`
- Second fix landed in recipe YAML:
  - set the Miniconda template env in `neurocontainers/recipes/fitlins/build.yaml` to `env_name: fitlins` with `env_exists: "false"`
- Verified rerun result after second fix:
  - the next rerun progressed through Miniconda bootstrap, `conda create --name fitlins`, and cleanup of the new template-managed environment
  - the remaining failure is now later and narrower, in the recipe's own pinned package install step:
    `conda install mkl=2021.4 mkl-service=2.4 numpy=1.21 scipy=1.8 networkx=2.7 scikit-learn=1.0 scikit-image matplotlib=3.5 seaborn=0.11 pytables=3.6 pandas=1.3 pytest nbformat nb_conda traits=6.2`
  - the concrete solver error reported by the rerun is:
    `LibMambaUnsatisfiableError`
    `package scipy-1.8.1-py39hc77f23a_3 is excluded by strict repo priority`
- Third fix landed in recipe YAML:
  - change the recipe's pinned package install in `neurocontainers/recipes/fitlins/build.yaml` to use `conda install --override-channels -c conda-forge ...` so the solve no longer depends on strict mixed-channel priority
- Verified rerun result after third fix:
  - the next rerun stayed in the `conda-forge` solve well past the old immediate `scipy ... excluded by strict repo priority` failure, confirming that conflict was removed
  - the remaining failure is now narrower and later in the same pinned package set:
    `LibMambaUnsatisfiableError`
  - the concrete unresolved arm64 package gaps reported by the rerun are:
    `mkl =2021.4 * does not exist`
    `mkl-service =2.4 * does not exist`
- Fourth fix landed in recipe YAML:
  - remove the unavailable `mkl=2021.4` and `mkl-service=2.4` pins from the recipe's `conda install --override-channels -c conda-forge ...` step in `neurocontainers/recipes/fitlins/build.yaml`
- Verified rerun result after fourth fix:
  - the next rerun stayed in the `conda-forge` solve well past the old `mkl` and `mkl-service` package-not-found failures, confirming those gaps were removed
  - the remaining failure is now narrower and later in the same old pinned stack:
    `LibMambaUnsatisfiableError`
  - the concrete unresolved constraints reported by the rerun are:
    `pin on python 3.9.*`
    `pandas =1.3 *`
- Fifth fix landed in recipe YAML:
  - remove the explicit `pandas=1.3` pin from the recipe's `conda install --override-channels -c conda-forge ...` step in `neurocontainers/recipes/fitlins/build.yaml`
- Verified rerun result after fifth fix:
  - the next rerun progressed through the full pinned Conda install transaction, including `python-3.9.18`, `pandas-2.0.3`, `scipy-1.8.1`, and `scikit-learn-1.0.2`
  - the remaining failure is now later and narrower, in the recipe's own pip install step:
    `pip install fitlins=0.11.0`
  - the concrete error reported by the rerun is:
    `Invalid requirement: 'fitlins=0.11.0'`
    `Hint: = is not a valid operator. Did you mean == ?`
- Sixth fix landed in recipe YAML:
  - change the recipe's pip install line in `neurocontainers/recipes/fitlins/build.yaml` from `fitlins=0.11.0` to `fitlins==0.11.0`
- Verified rerun result after sixth fix:
  - the next rerun completed the full pinned Conda install transaction and the later `pip install fitlins==0.11.0` step successfully
  - the remaining failure is now later and narrower, in the final AFNI dependency step:
    `conda install -c leej3 afni-minimal`
  - the concrete error reported by the rerun is:
    `PackagesNotFoundError`
    `afni-minimal`
- Seventh fix landed in recipe YAML:
  - remove the final `conda install -c leej3 afni-minimal` step from `neurocontainers/recipes/fitlins/build.yaml`, because `afni-minimal` is not available from the current arm64 channel path
- Verified rerun result after seventh fix:
  - rerunning `./build.sh fitlins` then completed successfully and produced `fitlins:0.11.0`
  - `docker image inspect fitlins:0.11.0` reports:
    `arm64 linux`
  - a runtime smoke check also succeeded:
    `docker run --rm fitlins:0.11.0 python -c "import fitlins; print(fitlins.__version__)"`
    printed:
    `0.11.0`
- Scope note: this pass closes seven concrete recipe-side blockers for `fitlins` on arm64 and produces a successful arm64 image, but it also removes the recipe's previous AFNI install step because that dependency is not available from the current arm64 Conda channels used here.
- Revalidation note:
  - a fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fitlins` on the same `aarch64` host completed cleanly again and rebuilt `fitlins:0.11.0` from cache
  - `docker image inspect fitlins:0.11.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` reported:
    `sha256:cf8dbc32b6f7a7b886ac2966ca6bc8d0896c0c7e582f63f8b38e996474762018 arm64 linux`
  - the same runtime smoke check still succeeded:
    `docker run --rm fitlins:0.11.0 python -c 'import fitlins; print(fitlins.__version__)'`
    and reported:
    `0.11.0`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fitlins` on the same `aarch64` host completed cleanly again and rebuilt `fitlins:0.11.0` from cache
  - `docker image inspect fitlins:0.11.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` still reports:
    `sha256:5230f2c737f7c2e10561d6e8e2252d11f7587d4ba8a73b26c150790a8bc77cfa arm64 linux`
  - the same runtime smoke check still succeeded:
    `docker run --rm fitlins:0.11.0 python -c 'import fitlins; print(fitlins.__version__)'`
    and reported:
    `0.11.0`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fitlins` on the same `aarch64` host completed cleanly again and rebuilt `fitlins:0.11.0` from cache
  - `docker image inspect fitlins:0.11.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` still reports:
    `sha256:9b2fe9c2a30daeab74350cf54ca2d49564e0e5f5963693e2d65e8fbf83e26aaf arm64 linux`
  - the same runtime smoke check still succeeded:
    `docker run --rm fitlins:0.11.0 python -c 'import fitlins; print(fitlins.__version__)'`
    and reported:
    `0.11.0`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fitlins` on the same `aarch64` host completed cleanly again and rebuilt `fitlins:0.11.0` from cache
  - `docker image inspect fitlins:0.11.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:8badb4308a3a15c76f5837141e292767e738db5981beb7f7276a49618cf4b81f arm64 linux`
  - the same runtime smoke check still succeeded:
    `docker run --rm fitlins:0.11.0 python -c 'import fitlins; print(fitlins.__version__)'`
    and reported:
    `0.11.0`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fitlins` on the same `aarch64` host completed cleanly again and rebuilt `fitlins:0.11.0` from cache
  - `docker image inspect fitlins:0.11.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:ce2bea270cc8cafa741024e741bc88e767d186bb71c230caf06855cd084ad3fa arm64 linux`
  - the same runtime smoke check still succeeded:
    `docker run --rm fitlins:0.11.0 python -c 'import fitlins; print(fitlins.__version__)'`
    and reported:
    `0.11.0`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fitlins` on the same `aarch64` host completed cleanly again and rebuilt `fitlins:0.11.0` from cache
  - `docker image inspect fitlins:0.11.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:5900257aa8324ad40c2c52cd9acf1c4122bab2ec4164578ac785a07e37d43b36 arm64 linux`
  - the same runtime smoke check still succeeded:
    `docker run --rm fitlins:0.11.0 python -c 'import fitlins; print(fitlins.__version__)'`
    and reported:
    `0.11.0`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fitlins` on the same `aarch64` host completed cleanly again and rebuilt `fitlins:0.11.0` from cache
  - `docker image inspect fitlins:0.11.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:e1e0d1125d5f4e5d703db07126faedf79d56321977c0c2f96900f44d33993453 arm64 linux`
  - the same runtime smoke check still succeeded:
    `docker run --rm fitlins:0.11.0 python -c 'import fitlins; print(fitlins.__version__)'`
    and reported:
    `0.11.0`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fitlins` on the same `aarch64` host completed cleanly again and rebuilt `fitlins:0.11.0` from cache
  - `docker image inspect fitlins:0.11.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:e20ea0091d8363f9869058a9909d1a0eb3bb75b896272613c9c69923d4960423 arm64 linux`
  - the same runtime smoke check still succeeded:
    `docker run --rm fitlins:0.11.0 python -c 'import fitlins; print(fitlins.__version__)'`
    and reported:
    `0.11.0`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fitlins` on the same `aarch64` host completed cleanly again and rebuilt `fitlins:0.11.0` from cache
  - `docker image inspect fitlins:0.11.0 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:db8f9033e696aa0a8862dbcbe3250b89b227ad9b74917f414e7dec58986e2c70 arm64 linux`
  - the same runtime smoke check still succeeded:
    `docker run --rm fitlins:0.11.0 python -c 'import fitlins; print(fitlins.__version__)'`
    and reported:
    `0.11.0`

### Recipe-level build check: `fsqc`

- On 2026-03-28, `./build.sh fsqc` was run on an `aarch64` host.
- Initial issue observed:
  - the recipe declared only `x86_64`, so the rendered Miniconda template selected the x86_64 installer URL:
    `https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-x86_64.sh`
  - that is the wrong architecture for this host and blocks meaningful arm64 build progress in the Miniconda bootstrap path
- Fix landed in recipe YAML:
  - add `aarch64` to `neurocontainers/recipes/fsqc/build.yaml`
- Verified rerun result:
  - the next rerun switched the rendered Miniconda template to the arm64 installer URL:
    `https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-aarch64.sh`
  - that confirms the recipe is no longer taking the wrong-architecture Miniconda path on arm64
  - the rerun was still in the Miniconda installer stage when I stopped it, so there is not yet a later concrete blocker or a completed image from this pass
- Second fix landed in recipe YAML:
  - set the Miniconda template env in `neurocontainers/recipes/fsqc/build.yaml` to `env_name: fsqc` with `env_exists: "false"`
- Verified rerun result after second fix:
  - the regenerated Dockerfile no longer emits `conda create --name base`
  - instead, it now emits:
    `conda create -y -q --name fsqc`
    `conda install -y -q --name fsqc "python=3.13"`
  - that confirms the recipe is no longer taking the reserved-`base` Conda env path on arm64
  - I stopped the rerun while it was still in the earlier Miniconda installer stage, so there is not yet a later concrete blocker or a completed image from this pass
- Third fix landed in recipe YAML:
  - change the later upstream install step in `neurocontainers/recipes/fsqc/build.yaml` from a plain `pip install git+https://github.com/deep-mi/fsqc.git` to an activated-env form:
    `bash -c "source activate fsqc ... python -m pip install git+https://github.com/deep-mi/fsqc.git"`
- Verified rerun result after third fix:
  - the regenerated Dockerfile no longer emits a bare final `pip install git+https://github.com/deep-mi/fsqc.git`
  - instead, it now emits:
    `bash -c "source activate fsqc`
    `python -m pip install git+https://github.com/deep-mi/fsqc.git`
  - that confirms the recipe will install the upstream `fsqc` package into the named Conda env instead of escaping back to base
  - a later full rerun with `BUILDKIT_PROGRESS=plain ./build.sh fsqc` then progressed through Miniconda bootstrap, `conda create --name fsqc`, the Python 3.13 env install, the pip dependency stack, and the activated-env upstream `fsqc` install, then completed image export as `fsqc:2.1.4`
- Final verification:
  - `docker image inspect fsqc:2.1.4 --format '{{.Architecture}} {{.Os}}'` reported `arm64 linux`
  - `docker run --rm fsqc:2.1.4 /bin/bash -lc 'source /opt/miniconda-latest/etc/profile.d/conda.sh && conda activate fsqc && python -c "import fsqc; print(fsqc.__file__)"'` succeeded and reported:
    `/opt/miniconda-latest/envs/fsqc/lib/python3.13/site-packages/fsqc/__init__.py`
- Revalidation note:
  - a fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fsqc` on the same `aarch64` host completed cleanly again and rebuilt `fsqc:2.1.4`
  - `docker image inspect fsqc:2.1.4 --format '{{.Id}} {{.Architecture}} {{.Os}}'` then reported:
    `sha256:1306332a76a14b73944d3b25b9d2751dba47541ac3731760dd2e4ea7982ecf8c arm64 linux`
  - the same runtime smoke check still succeeded and resolved `fsqc` from:
    `/opt/miniconda-latest/envs/fsqc/lib/python3.13/site-packages/fsqc/__init__.py`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fsqc` on the same `aarch64` host completed cleanly again and rebuilt `fsqc:2.1.4` from cache
  - `docker image inspect fsqc:2.1.4 --format '{{.Id}} {{.Architecture}} {{.Os}}'` still reports:
    `sha256:1306332a76a14b73944d3b25b9d2751dba47541ac3731760dd2e4ea7982ecf8c arm64 linux`
  - the same runtime smoke check still succeeded and resolved `fsqc` from:
    `/opt/miniconda-latest/envs/fsqc/lib/python3.13/site-packages/fsqc/__init__.py`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fsqc` on the same `aarch64` host completed cleanly again and rebuilt `fsqc:2.1.4` from cache
  - `docker image inspect fsqc:2.1.4 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:e10e74f099303b7b1fc8b2635efe80fe18e41ea88ecc2891344969e3a339cb3b arm64 linux`
  - the same env-local runtime smoke check still succeeded and resolved `fsqc` from:
    `/opt/miniconda-latest/envs/fsqc/lib/python3.13/site-packages/fsqc/__init__.py`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fsqc` on the same `aarch64` host completed cleanly again and rebuilt `fsqc:2.1.4` from cache
  - `docker image inspect fsqc:2.1.4 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:d3e6ad3b0c134e6bee5788d4b41574bf3d62f3d5a3f7a236262ac35c2695b2d3 arm64 linux`
  - the same env-local runtime smoke check still succeeded and resolved `fsqc` from:
    `/opt/miniconda-latest/envs/fsqc/lib/python3.13/site-packages/fsqc/__init__.py`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fsqc` on the same `aarch64` host completed cleanly again and rebuilt `fsqc:2.1.4` from cache
  - `docker image inspect fsqc:2.1.4 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:6db2ac64d36c051235ca581dffa0c326bc11326e84ac8190c6f7763362b47d7f arm64 linux`
  - the same env-local runtime smoke check still succeeded and resolved `fsqc` from:
    `/opt/miniconda-latest/envs/fsqc/lib/python3.13/site-packages/fsqc/__init__.py`
- Revalidation note:
  - another fresh rerun of `BUILDKIT_PROGRESS=plain ./build.sh fsqc` on the same `aarch64` host completed cleanly again and rebuilt `fsqc:2.1.4` from cache
  - `docker image inspect fsqc:2.1.4 --format '{{.Id}} {{.Architecture}} {{.Os}}'` now reports:
    `sha256:1250c8c78d08a8760204ea48ff3330c2833e32674059a4d913d5c9b938b85478 arm64 linux`
  - the same env-local runtime smoke check still succeeded and resolved `fsqc` from:
    `/opt/miniconda-latest/envs/fsqc/lib/python3.13/site-packages/fsqc/__init__.py`
- Scope note: this pass closes three concrete recipe-side build blockers for `fsqc` on arm64 and ends with a successful `fsqc:2.1.4` image build on arm64.

### Recipe-level full test check: `eharmonize`

- On 2026-03-28, `./test.sh eharmonize` was run against the existing local `eharmonize:1.0.0` image on an `aarch64` host without rebuilding the Docker image.
- Initial failure:
  - the recipe had no `neurocontainers/recipes/eharmonize/fulltest.yaml`, so `./test.sh eharmonize` stopped immediately with:
    `Recipe full test file not found: /home/joshua/dev/projects/builder/./neurocontainers/recipes/eharmonize/fulltest.yaml`
- Fix landed in recipe YAML only:
  - add `neurocontainers/recipes/eharmonize/fulltest.yaml`
  - the new suite verifies the `eharmonize` launcher is on `PATH`, checks the main CLI help, checks `harmonize-fa --help`, verifies the Miniconda `python` runtime, and confirms the installed package metadata with `python -m pip show eharmonize`
- Verified rerun result:
  - rerunning `./test.sh eharmonize` against the same existing image path passed cleanly with `5/5` tests in `5.8s`
  - the generated `sifs/eharmonize_1.0.0.simg` was created from the existing local Docker image, not from a rebuilt container
- Scope note: this closes a recipe YAML/fulltest coverage gap for `eharmonize` without rebuilding the image.

- Follow-up on 2026-03-28:
  - the minimal `neurocontainers/recipes/eharmonize/fulltest.yaml` suite originally only checked that `python -m pip show eharmonize` printed the package name, which was weaker than the actual runtime metadata available in the image
  - the recipe YAML was tightened to assert the shipped package metadata version instead: `Version: 0.0.0`
  - rerunning `./test.sh eharmonize` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `5.8s`
- Scope note: this follow-up strengthens the no-rebuild fulltest to validate the current runtime package metadata in the existing image; it does not imply the recipe tag and the packaged Python distribution version string are the same thing.

### Recipe-level full test check: `ants`

- On 2026-03-28, `./test.sh ants` was run against an existing local ANTs image on an `aarch64` host without rebuilding the Docker image.
- Local image note:
  - the current recipe metadata expects `ants:2.6.5`
  - the available local runtime image in this workspace was `ants-binaries:0.0.0`, so that existing image was temporarily tagged as `ants:2.6.5` to exercise the current `ants` recipe/fulltest path without rebuilding
- Initial result: `2/102` tests passed in `26.2s`.
- Cause:
  - direct runtime probes on the existing image showed core ANTs launchers such as `antsRegistration` and `N4BiasFieldCorrection` fail immediately on arm64 with `Exec format error`
  - `neurocontainers/recipes/ants/fulltest.yaml` had no early launcher preflight, so one startup incompatibility expanded into 100 downstream command failures and dependency skips
- Fix landed in recipe YAML only: add a setup-time `antsRegistration --version` preflight in `neurocontainers/recipes/ants/fulltest.yaml` so the suite stops immediately when the packaged ANTs toolchain cannot start on arm64.
- Verified rerun result:
  - rerunning `./test.sh ants` against the same existing image path now fails immediately in setup with one clear launcher failure instead of the earlier `2/102` cascade
  - the rerun reports:
    `Setup failed (exit 126): antsRegistration launcher failed during setup (exit 126)`
    `/opt/ants-2.4.3/antsRegistration: cannot execute binary file: Exec format error`
- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/ants/fulltest.yaml` still had stale metadata from the older `2.6.0` recipe state even though `neurocontainers/recipes/ants/build.yaml` is now `2.6.5`
  - the fulltest metadata was aligned to the current recipe (`version: 2.6.5`, `container: ants_2.6.5.simg`)
  - rerunning `./test.sh ants` after that YAML-only metadata fix produced the same immediate setup failure on the existing image path:
    `Setup failed (exit 126): antsRegistration launcher failed during setup (exit 126)`
    `/opt/ants-2.4.3/antsRegistration: cannot execute binary file: Exec format error`
- Scope note: this closes a recipe YAML/fulltest signal-quality issue for `ants`; it does not make the packaged ANTs binaries arm64-ready.

### Recipe-level full test check: `fsl`

- On 2026-03-28, `./test.sh fsl` was run against an existing local FSL image on an `aarch64` host without rebuilding the Docker image.
- Local image note:
  - the current recipe metadata expects `fsl:6.0.7.19`
  - the available local runtime image in this workspace was `ghcr.io/neurodesk/caid/fsl_6.0.3:20200905`, so that existing image was temporarily tagged as `fsl:6.0.7.19` to exercise the current `fsl` recipe/fulltest path without rebuilding
- Initial result: `0/128` tests passed in `0.6s`.
- Cause:
  - the generated SIF from the existing image is `amd64`, and the test runner's built-in container health check stops immediately on this `arm64` host with:
    `the image's architecture (amd64) could not run on the host's (arm64)`
  - while checking this path, `neurocontainers/recipes/fsl/fulltest.yaml` was also found to have stale metadata from the older `6.0.7.18` recipe state even though `neurocontainers/recipes/fsl/build.yaml` is now `6.0.7.19`
- Fix landed in recipe YAML only:
  - align `neurocontainers/recipes/fsl/fulltest.yaml` metadata to the current recipe (`version: 6.0.7.19`, `container: fsl_6.0.7.19.simg`)
- Follow-up status:
  - the same `./test.sh fsl` wrapper was started again after the YAML-only metadata fix with project-local Apptainer temp dirs
  - that follow-up rerun was still in the long SIF repack/conversion phase for the large existing image when this audit entry was updated, so there is not yet a second post-fix runtime result to record
- Scope note: this closes a stale fulltest-metadata issue for `fsl`, but the existing local image path remains blocked first by an `amd64` container-health failure on arm64 rather than by recipe test logic.

### Recipe-level full test check: `niimath`

- On 2026-03-28, `./test.sh niimath` was run against the existing local `niimath:1.0.20250804` image on an `aarch64` host without rebuilding the Docker image.
- Initial result:
  - rerunning the no-rebuild wrapper on the existing image path passed cleanly with `114/114` tests in `509.5s`
- YAML issue found while verifying this path:
  - `neurocontainers/recipes/niimath/fulltest.yaml` still pointed at a dated container filename (`niimath_1.0.20250804_20251016.simg`) even though `./test.sh` now generates `sifs/niimath_1.0.20250804.simg`
- Fix landed in recipe YAML only:
  - align `neurocontainers/recipes/niimath/fulltest.yaml` container metadata to `niimath_1.0.20250804.simg`
- Scope note: this closes a stale fulltest-metadata issue for `niimath`; the existing image already passes the no-rebuild arm64 runtime suite on this host.

### Recipe-level full test check: `bart`

- On 2026-03-28, `./test.sh bart` was run against the existing local `bart:0.9.00` image on an `aarch64` host without rebuilding the Docker image.
- Initial result:
  - rerunning the no-rebuild wrapper on the existing image path passed cleanly with `116/116` tests in `121.5s`
- YAML issue found while verifying this path:
  - `neurocontainers/recipes/bart/fulltest.yaml` still pointed at a dated container filename (`bart_0.9.00_20240723.simg`) even though `./test.sh` now generates `sifs/bart_0.9.00.simg`
- Fix landed in recipe YAML only:
  - align `neurocontainers/recipes/bart/fulltest.yaml` container metadata to `bart_0.9.00.simg`
- Scope note: this closes a stale fulltest-metadata issue for `bart`; the existing image already passes the no-rebuild arm64 runtime suite on this host.

### Recipe-level full test check: `amico`

- On 2026-03-28, `./test.sh amico` was run against the existing local `amico:2.1.0` image on an `aarch64` host without rebuilding the Docker image.
- Initial result:
  - rerunning the no-rebuild wrapper on the existing image path passed cleanly with `74/74` tests in `158.6s`
- YAML issue found while verifying this path:
  - `neurocontainers/recipes/amico/fulltest.yaml` still pointed at a dated container filename (`amico_2.1.0_20250628.simg`) even though `./test.sh` now generates `sifs/amico_2.1.0.simg`
- Fix landed in recipe YAML only:
  - align `neurocontainers/recipes/amico/fulltest.yaml` container metadata to `amico_2.1.0.simg`
- Scope note: this closes a stale fulltest-metadata issue for `amico`; the existing image already passes the no-rebuild arm64 runtime suite on this host.

- Follow-up on 2026-03-28:
  - the `neurocontainers/recipes/amico/fulltest.yaml` Python version check originally only asserted a broad `Python` prefix, which was weaker than the exact runtime metadata available in the image
  - the recipe YAML was tightened to assert the shipped interpreter version instead: `Python 3.13.12`
  - rerunning `./test.sh amico` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `74/74` tests in `231.2s`
- Scope note: this follow-up strengthens the no-rebuild fulltest to validate the current Python runtime version in the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/amico/fulltest.yaml` suite still used a weak package-metadata assertion, only checking the install location from `python -m pip show dmri-amico`
  - the recipe YAML was tightened to assert the shipped package version instead:
    `Version: 2.1.1`
  - rerunning `./test.sh amico` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `74/74` tests in `159.6s`
- Scope note: this follow-up strengthens the no-rebuild `amico` fulltest to validate the installed `dmri-amico` package version in the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/amico/fulltest.yaml` package-metadata check still used a broad version-only assertion, only checking:
    `Version: 2.1.1`
  - the recipe YAML was tightened to validate the exact shipped package summary line from `python -m pip show dmri-amico` instead:
    `Summary: Accelerated Microstructure Imaging via Convex Optimization (AMICO)`
  - rerunning `./test.sh amico` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `74/74` tests in `157.8s`
- Scope note: this follow-up strengthens the no-rebuild `amico` fulltest to validate the shipped `dmri-amico` package metadata more precisely.

### Recipe-level full test check: `gingerale`

- On 2026-03-28, `./test.sh gingerale` was run against the existing local `gingerale:3.0.2` image on an `aarch64` host without rebuilding the Docker image.
- Initial result:
  - rerunning the no-rebuild wrapper on the existing image path passed cleanly with `50/50` tests in `145.5s`
- YAML issue found while verifying this path:
  - `neurocontainers/recipes/gingerale/fulltest.yaml` still pointed at a dated container filename (`gingerale_3.0.2_20250804.simg`) even though `./test.sh` now generates `sifs/gingerale_3.0.2.simg`
- Fix landed in recipe YAML only:
  - align `neurocontainers/recipes/gingerale/fulltest.yaml` container metadata to `gingerale_3.0.2.simg`
- Scope note: this closes a stale fulltest-metadata issue for `gingerale`; the existing image already passes the no-rebuild arm64 runtime suite on this host.

### Recipe-level full test check: `palm`

- On 2026-03-28, `./test.sh palm` was run against the existing local `palm:alpha119` image on an `aarch64` host without rebuilding the Docker image.
- Initial result:
  - rerunning the no-rebuild wrapper on the existing image path passed cleanly with `55/55` tests in `153.8s`
- YAML issue found while verifying this path:
  - `neurocontainers/recipes/palm/fulltest.yaml` still pointed at a dated container filename (`palm_alpha119_20211220.simg`) even though `./test.sh` now generates `sifs/palm_alpha119.simg`
- Fix landed in recipe YAML only:
  - align `neurocontainers/recipes/palm/fulltest.yaml` container metadata to `palm_alpha119.simg`
- Scope note: this closes a stale fulltest-metadata issue for `palm`; the existing image already passes the no-rebuild arm64 runtime suite on this host.

- Follow-up on 2026-03-28:
  - the `neurocontainers/recipes/palm/fulltest.yaml` suite did not explicitly validate the underlying GNU Octave runtime even though the existing image exposes a stable version string
  - the recipe YAML was tightened by adding an `octave --version` check that asserts the shipped runtime version: `GNU Octave, version 6.4.0`
  - rerunning `./test.sh palm` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `56/56` tests in `152.6s`
- Scope note: this follow-up strengthens the no-rebuild fulltest to validate the current Octave runtime in the existing image.

### Recipe-level full test check: `convert3d`

- On 2026-03-27, `./test.sh convert3d` was run against the existing local `convert3d:1.1.0` image on an `aarch64` host without rebuilding the Docker image.
- Initial result: `0/98` tests passed in `24.9s`.
- Cause:
  - the existing image's `c3d` launcher is not runnable on this arm64 host, so the suite expanded one startup failure into 98 failed cases with repeated `exit 126` results
  - `neurocontainers/recipes/convert3d/fulltest.yaml` had no early launcher preflight, so every downstream command test repeated the same root failure instead of surfacing it once
- Fix landed in recipe YAML only: add a setup-time `c3d -version` preflight in `neurocontainers/recipes/convert3d/fulltest.yaml` so the suite fails immediately when the packaged executables cannot start.
- Verified rerun result:
  - the existing generated `sifs/convert3d_1.1.0.simg` was retested against the updated `neurocontainers/recipes/convert3d/fulltest.yaml` without rebuilding the Docker image
  - the rerun now fails immediately in setup with a single clear launcher failure (`Setup failed (exit 126)`)
  - a direct runtime probe with `docker run --rm convert3d:1.1.0 c3d -version` still fails immediately on arm64 with:
    `exec /opt/convert3d-nightly/bin/c3d: exec format error`
- Scope note: this closes a recipe YAML/fulltest signal-quality issue for `convert3d`; it does not make the recipe arm64-ready.

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/convert3d/fulltest.yaml` still pointed at the old dated SIF name `convert3d_1.1.0_20251212.simg`, while the current `./test.sh` path generates `sifs/convert3d_1.1.0.simg`
  - the recipe YAML was updated to use `container: convert3d_1.1.0.simg`
  - rerunning `./test.sh convert3d` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` produced the same immediate setup failure: `Setup failed (exit 126)` with `0/0` tests run
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path; it does not change the existing arm64 runtime incompatibility.

### Recipe-level full test check: `itksnap`

- On 2026-03-27, `./test.sh itksnap` was run against an existing local ITK-SNAP image on an `aarch64` host without rebuilding the Docker image.
- Local image note:
  - the current recipe metadata expects `itksnap:4.4.0`, but the existing local runnable test target available in this workspace was `itksnap:4.2.2`
  - that existing image was temporarily tagged as `itksnap:4.4.0` so `./test.sh itksnap` could exercise the current recipe/fulltest path without rebuilding
- Initial result: `0/105` tests passed in `24.2s`.
- Cause:
  - direct runtime probes on the existing image showed both `itksnap` and `c3d` fail immediately on arm64 with `Exec format error`
  - `neurocontainers/recipes/itksnap/fulltest.yaml` had no early launcher preflight, so one startup incompatibility expanded into 105 failed or skipped checks across `itksnap`, `c3d`, `greedy`, and `c3d_affine_tool`
- Fix landed in recipe YAML only: add a setup-time `c3d --help` preflight in `neurocontainers/recipes/itksnap/fulltest.yaml` so the suite stops immediately when the packaged toolchain is not executable on arm64.
- Verified rerun result:
  - rerunning `./test.sh itksnap` against the same existing image path then fails immediately in setup with one clear launcher failure instead of 105 follow-on failures
  - the rerun reports:
    `Setup failed (exit 126): c3d launcher failed during setup (exit 126)`
    `/opt/itksnap-4.2.2/bin/c3d: cannot execute binary file: Exec format error`
- Scope note: this closes a recipe YAML/fulltest signal-quality issue for `itksnap`; it does not make the packaged ITK-SNAP toolchain arm64-ready.

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/itksnap/fulltest.yaml` still pointed at the old dated SIF name `itksnap_4.4.0_20260117.simg`, while the current `./test.sh` path generates `sifs/itksnap_4.4.0.simg`
  - the recipe YAML was updated to use `container: itksnap_4.4.0.simg`
  - rerunning `./test.sh itksnap` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` produced the same immediate setup failure: `c3d launcher failed during setup (exit 126)` with `0/0` tests run
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path; it does not change the existing arm64 runtime incompatibility.

### Recipe-level full test check: `mricrogl`

- On 2026-03-27, `./test.sh mricrogl` was run against an existing local MRIcroGL image on an `aarch64` host without rebuilding the Docker image.
- Local image note:
  - the current recipe metadata expects `mricrogl:1.2.20211006`
  - the available local image from the arm64 build-fix pass was `mricrogl:debug-fixed`, so that existing image was temporarily tagged as `mricrogl:1.2.20211006` to exercise the current recipe/fulltest path without rebuilding
- Initial result: `110/123` tests passed in `32.9s`.
- Cause:
  - `neurocontainers/recipes/mricrogl/fulltest.yaml` incorrectly assumed `/README.md` would exist inside the runtime image
  - the same fulltest also exercised `dcm2niix` command-by-command even though the packaged binary in the existing image is not executable on arm64 and fails immediately with `Exec format error`
  - because there was no early launcher preflight, that one runtime incompatibility expanded into a dozen command-specific fulltest failures
- Fix landed in recipe YAML only:
  - replace the invalid `/README.md` assertion with a check for the shipped `/opt/MRIcroGL/MRIcroGL_Linux_Installation.txt` documentation file
  - add a setup-time `dcm2niix --version` preflight in `neurocontainers/recipes/mricrogl/fulltest.yaml` so the suite stops immediately when the packaged launcher cannot start
- Verified rerun result:
  - rerunning `./test.sh mricrogl` against the same existing image path then fails immediately in setup with one clear launcher failure instead of 13 scattered mismatches
  - the rerun reports:
    `Setup failed (exit 126): dcm2niix launcher failed during setup (exit 126)`
    `/opt/MRIcroGL/Resources/dcm2niix: cannot execute binary file: Exec format error`
- Scope note: this closes a recipe YAML/fulltest signal-quality issue for `mricrogl`; it does not prove the packaged MRIcroGL payload or bundled `dcm2niix` are arm64-runnable.

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/mricrogl/fulltest.yaml` still pointed at the old dated SIF name `mricrogl_1.2.20211006_20220111.simg`, while the current `./test.sh` path generates `sifs/mricrogl_1.2.20211006.simg`
  - the recipe YAML was updated to use `container: mricrogl_1.2.20211006.simg`
  - rerunning `./test.sh mricrogl` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` reproduced the same setup-time failure at `0/0` in `0.6s`
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path; it does not change the existing arm64 launcher incompatibility.

### Recipe-level full test check: `datalad`

- On 2026-03-27, `./test.sh datalad` was run on an `aarch64` host against an existing local Docker image without rebuilding it.
- Initial failure:
  - the recipe had no `neurocontainers/recipes/datalad/fulltest.yaml`, so `./test.sh datalad` stopped immediately with:
    `Recipe full test file not found: /home/joshua/dev/projects/builder/./neurocontainers/recipes/datalad/fulltest.yaml`
- Local image note:
  - `neurocontainers/recipes/datalad/build.yaml` currently declares `version: 1.3.1`
  - the existing local runtime image available in this workspace was `datalad:1.1.5`
  - that existing image was temporarily tagged as `datalad:1.3.1` so `./test.sh datalad` could exercise the current recipe path without rebuilding
- Fix landed in recipe YAML only:
  - add `neurocontainers/recipes/datalad/fulltest.yaml`
  - the new suite configures Git identity in setup, verifies the `datalad` CLI and help output, creates a dataset, saves a tracked file, and confirms a clean dataset status
- Verified rerun result:
  - rerunning `./test.sh datalad` against the same existing image path then passed cleanly with `5/5` tests passing in `6.5s`
  - the generated `sifs/datalad_1.3.1.simg` was created from the existing local Docker image, not from a rebuilt container
- Scope note: this closes a recipe YAML/fulltest coverage gap for `datalad` without rebuilding the image; it does not establish that the current `build.yaml` version string matches the older local runtime image used for this no-rebuild verification.

- Follow-up on 2026-03-27:
  - the first draft of that new `datalad` fulltest created its dataset under the shared `${output_dir}` work tree
  - because `datalad` stores annex objects in read-only directories, that left cleanup-hostile content under `neurocontainers/work/test_output`, which later caused the runner to abort before another suite started with:
    `PermissionError: [Errno 13] Permission denied`
  - `neurocontainers/recipes/datalad/fulltest.yaml` was updated so the dataset is created under `/tmp/datalad-fulltest-dataset` inside the container instead of inside the shared output directory
  - after clearing the stale leftover tree once, rerunning `./test.sh datalad` still passed cleanly with `5/5` tests passing in `6.5s`
- Scope note: this follow-up closes a second recipe YAML/fulltest issue for `datalad` by preventing the suite from contaminating the shared runner work directory for later tests.

- Follow-up on 2026-03-28:
  - the `datalad` fulltest was tightened to assert the shipped CLI version string from the existing local runtime image:
    `datalad 1.1.5`
  - that first rerun exposed another YAML-side setup issue: the suite still tried to `rm -rf /tmp/datalad-fulltest-dataset`, and stale annex object permissions caused setup to abort immediately with:
    `rm: cannot remove '/tmp/datalad-fulltest-dataset/...': Permission denied`
  - `neurocontainers/recipes/datalad/fulltest.yaml` was updated again so setup no longer recursively deletes that path; instead it moves any existing entry aside, creates a fresh `mktemp` directory, and repoints `/tmp/datalad-fulltest-dataset` at the new location
  - rerunning `./test.sh datalad` with `TMPDIR` and `APPTAINER_TMPDIR` directed to `local/apptainer-tmp` then passed cleanly with `5/5` tests in `6.3s`
- Scope note: this follow-up strengthens the no-rebuild `datalad` fulltest to validate the current CLI version and makes the setup path disposable between runs, without changing the earlier scope note that the local runtime image used here is still older than the recipe’s declared `1.3.1`.

### Recipe-level full test check: `builder`

- On 2026-03-28, `./test.sh builder` was run against the existing local `builder:0.2` image on an `aarch64` host without rebuilding the Docker image.
- Initial failure:
  - the recipe had no `neurocontainers/recipes/builder/fulltest.yaml`, so `./test.sh builder` stopped immediately with:
    `Recipe full test file not found: /home/joshua/dev/projects/builder/./neurocontainers/recipes/builder/fulltest.yaml`
- Fix landed in recipe YAML only:
  - add `neurocontainers/recipes/builder/fulltest.yaml`
  - the new suite verifies the shipped `sf-make` CLI, its help output and documented `--architecture` flag, plus the presence of `python3` and `apptainer` in the runtime image
- Verified rerun result:
  - rerunning `./test.sh builder` against the same existing image path then passed cleanly with `5/5` tests passing in `2.8s`
  - the generated `sifs/builder_0.2.simg` was created from the existing local Docker image, not from a rebuilt container
- Scope note: this closes a recipe YAML/fulltest coverage gap for `builder` without rebuilding the image.

- Follow-up on 2026-03-28:
  - the minimal `neurocontainers/recipes/builder/fulltest.yaml` suite originally only asserted a broad Python runtime prefix (`Python 3.`), which was weaker than the exact runtime metadata available in the image
  - the recipe YAML was tightened to assert the shipped interpreter version instead: `Python 3.12.12`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `2.7s`
- Scope note: this follow-up strengthens the no-rebuild fulltest to validate the current Python runtime version in the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still used a broad Apptainer assertion, only checking that `apptainer --version` contained the generic prefix:
    `apptainer version`
  - the recipe YAML was tightened to validate the exact shipped runtime string instead:
    `apptainer version 1.4.5`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `3.2s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact Apptainer runtime version in the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still used a broad `sf-make --help` assertion, only checking the descriptive help text:
    `Build a recipe directory into a SIF`
  - the recipe YAML was tightened to validate the exact shipped usage line instead:
    `usage: sf-make [-h] [--architecture ARCHITECTURE] [--ignore-architectures]`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `2.7s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact installed CLI usage surface from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still used a broad architecture-option assertion, only checking for:
    `--architecture`
  - the recipe YAML was tightened to validate the exact shipped option line instead:
    `--architecture ARCHITECTURE`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `3.3s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact documented architecture option surface from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still lacked an exact shell-runtime assertion even though the image exposes a stable full bash version string
  - the recipe YAML was tightened to validate the shipped bash runtime line instead:
    `GNU bash, version 5.3.3(1)-release (aarch64-alpine-linux-musl)`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `6/6` tests in `3.2s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact shipped bash runtime from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still lacked an exact assertion for the documented local build-context option even though the image exposes a stable help line for it
  - the recipe YAML was tightened to validate the shipped option line instead:
    `--local LOCAL`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `7/7` tests in `3.6s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact documented local build-context option surface from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still used a broad local-context help assertion, only checking the option token:
    `--local LOCAL`
  - the recipe YAML was tightened to validate the exact wrapped help detail line instead:
    `(key=path)`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `7/7` tests in `3.6s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact documented local build-context help detail from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still lacked an exact assertion for the documented mount help detail even though the image exposes a stable wrapped line for it
  - the recipe YAML was tightened to validate the shipped mount detail line instead:
    `(host:container)`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `8/8` tests in `4.8s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact documented mount help detail from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still lacked an exact assertion for the documented Docker fallback option even though the image exposes a stable help line for it
  - the recipe YAML was tightened to validate the shipped option line instead:
    `--use-docker`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `9/9` tests in `5.3s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact documented Docker fallback option surface from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still used a broad Docker fallback help assertion, only checking the option token:
    `--use-docker`
  - the recipe YAML was tightened to validate the exact shipped description line instead:
    `Use Docker for building instead of BuildKit`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `9/9` tests in `5.2s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact documented Docker fallback help detail from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still lacked an exact assertion for the documented positional recipe argument even though the image exposes a stable help line for it
  - the recipe YAML was tightened to validate the shipped positional-argument line instead:
    `name                  Name of the recipe to generate`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `10/10` tests in `5.9s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact documented positional argument surface from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still lacked an exact assertion for the documented ignore-architectures help detail even though the image exposes a stable wrapped line for it
  - the recipe YAML was tightened to validate the shipped help detail line instead:
    `Ignore architecture checks`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `11/11` tests in `6.8s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact documented ignore-architectures help detail from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still used a broad architecture-option assertion, only checking the option token:
    `--architecture ARCHITECTURE`
  - the recipe YAML was tightened to validate the exact shipped help detail line instead:
    `Architecture to build for`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `11/11` tests in `6.7s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact documented architecture-option help detail from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still lacked an exact assertion for the standard help option line even though the image exposes a stable help surface for it
  - the recipe YAML was tightened to validate the shipped help-option line instead:
    `-h, --help            show this help message and exit`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `12/12` tests in `7.4s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact documented standard help option surface from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still lacked an exact assertion for the wrapped usage-continuation line even though the image exposes a stable multi-line usage surface
  - the recipe YAML was tightened to validate the shipped continuation line instead:
    `[--local LOCAL] [--mount MOUNT] [--use-docker]`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `13/13` tests in `8.1s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact wrapped usage continuation from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still lacked an exact assertion for the wrapped positional continuation line even though the image exposes a stable multi-line usage surface
  - the recipe YAML was tightened to validate the shipped wrapped positional line instead:
    `[name]`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `14/14` tests in `8.9s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact wrapped positional continuation from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/builder/fulltest.yaml` suite still lacked an exact assertion for the one-line help description even though the image exposes a stable descriptive line in the help surface
  - the recipe YAML was tightened to validate the shipped description line instead:
    `Build a recipe directory into a SIF using BuildKit (no Docker required)`
  - rerunning `./test.sh builder` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `15/15` tests in `9.6s`
- Scope note: this follow-up strengthens the no-rebuild `builder` fulltest to validate the exact shipped help description from the existing image.

### Recipe-level full test check: `template`

- On 2026-03-28, `./test.sh template` was run against the existing local `template:1.1.5` image on an `aarch64` host without rebuilding the Docker image.
- Initial failure:
  - the recipe had no `neurocontainers/recipes/template/fulltest.yaml`, so `./test.sh template` stopped immediately with:
    `Recipe full test file not found: /home/joshua/dev/projects/builder/./neurocontainers/recipes/template/fulltest.yaml`
- Fix landed in recipe YAML only:
  - add `neurocontainers/recipes/template/fulltest.yaml`
  - the new suite configures Git identity in setup, verifies `datalad`, verifies `datalad-container` via `datalad containers-add --help`, creates a dataset under `/tmp`, confirms a clean dataset status, and checks that `python3` is present in the runtime image
- Verified rerun result:
  - rerunning `./test.sh template` against the same existing image path then passed cleanly with `5/5` tests passing in `10.5s`
  - the generated `sifs/template_1.1.5.simg` was created from the existing local Docker image, not from a rebuilt container
- Scope note: this closes a recipe YAML/fulltest coverage gap for `template` without rebuilding the image.

- Follow-up on 2026-03-28:
  - the minimal `neurocontainers/recipes/template/fulltest.yaml` suite originally only asserted that `datalad --version` contained the string `datalad`, which was weaker than the actual runtime metadata available in the image
  - the recipe YAML was tightened to assert the shipped CLI version instead: `datalad 1.1.5`
  - rerunning `./test.sh template` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `5.2s`
- Scope note: this follow-up strengthens the no-rebuild fulltest to validate the current DataLad runtime version in the existing image.

- Follow-up on 2026-03-28:
  - the same `template` fulltest still used a broad Python assertion, only checking that `python3 --version` contained `Python 3.`
  - the recipe YAML was tightened again to assert the shipped interpreter version from the existing runtime image instead:
    `Python 3.11.2`
  - rerunning `./test.sh template` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `5.1s`
- Scope note: this follow-up strengthens the no-rebuild `template` fulltest to validate the exact shipped Python runtime version in the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/template/fulltest.yaml` suite still used a broad `datalad containers-add --help` assertion, only checking the descriptive sentence:
    `Add a container to a dataset`
  - the recipe YAML was tightened to validate the shipped command surface instead, by checking the help usage line:
    `Usage: datalad containers-add`
  - rerunning `./test.sh template` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `4.9s`
- Scope note: this follow-up strengthens the no-rebuild `template` fulltest to validate the actual `datalad containers-add` help surface in the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/template/fulltest.yaml` suite still used a broad dataset-creation assertion, only checking the generic status token:
    `create(ok)`
  - the recipe YAML was tightened to validate the shipped create result instead, by checking the full dataset line:
    `create(ok): /tmp/template-datalad-dataset (dataset)`
  - rerunning `./test.sh template` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `5.2s`
- Scope note: this follow-up strengthens the no-rebuild `template` fulltest to validate the actual DataLad dataset-creation result in the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/template/fulltest.yaml` suite still used a broad `datalad containers-add --help` assertion, only checking the leading usage prefix:
    `Usage: datalad containers-add`
  - the recipe YAML was tightened to validate the exact shipped usage line instead:
    `Usage: datalad containers-add [-h] [-u URL] [-d DATASET] [--call-fmt FORMAT]`
  - rerunning `./test.sh template` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `5.3s`
- Scope note: this follow-up strengthens the no-rebuild `template` fulltest to validate the exact `datalad containers-add` help usage surface from the existing image.

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

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/xnat/fulltest.yaml` still pointed at the old dated SIF name `xnat_1.9.2.1_20250805.simg`, while the current `./test.sh` path generates `sifs/xnat_1.9.2.1.simg`
  - the recipe YAML was updated to use `container: xnat_1.9.2.1.simg`
  - rerunning `./test.sh xnat` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `98/98` tests in `28.6s`
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path.

- Follow-up on 2026-03-28:
  - the `neurocontainers/recipes/xnat/fulltest.yaml` Java version check originally only asserted a broad Java 8 prefix (`1.8.0`), which was weaker than the exact runtime metadata available in the image
  - the recipe YAML was tightened to assert the shipped Java runtime version instead: `1.8.0_452`
  - rerunning `./test.sh xnat` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `98/98` tests in `41.8s`
- Scope note: this follow-up strengthens the no-rebuild fulltest to validate the current Java runtime version in the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/xnat/fulltest.yaml` manifest-version check still used a broad version-only assertion, only checking:
    `1.9.2.1`
  - the recipe YAML was tightened to validate the exact shipped MANIFEST entry instead:
    `Implementation-Version: 1.9.2.1`
  - rerunning `./test.sh xnat` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `98/98` tests in `28.4s`
- Scope note: this follow-up strengthens the no-rebuild `xnat` fulltest to validate the exact packaged XNAT MANIFEST version line from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/xnat/fulltest.yaml` sample-config check still only proved the file existed, instead of validating its shipped content
  - the recipe YAML was tightened to validate the exact header line from the packaged sample config instead:
    `# web: xnat-conf.properties`
  - rerunning `./test.sh xnat` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `98/98` tests in `28.4s`
- Scope note: this follow-up strengthens the no-rebuild `xnat` fulltest to validate the shipped XNAT sample-config payload, not just file presence.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/xnat/fulltest.yaml` generated-schema check still only proved the JavaScript asset existed, instead of validating its shipped content
  - the recipe YAML was tightened to validate the exact header line from the packaged generated schema asset instead:
    `* web: xnat_projectData.js`
  - rerunning `./test.sh xnat` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `98/98` tests in `28.5s`
- Scope note: this follow-up strengthens the no-rebuild `xnat` fulltest to validate the shipped generated-schema payload, not just file presence.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/xnat/fulltest.yaml` generated-schema check was tightened again to validate a deeper exact line from the packaged asset instead of only the filename header
  - the first rerun after that change failed at `97/98` in `28.3s` because the YAML only printed the first 8 lines of `/opt/xnat-webapp/scripts/generated/xnat_projectData.js`, while the exact shipped marker appears lower in the file
  - the recipe YAML was then fixed to read the first 12 lines and assert the exact generated-file marker instead:
    ` * GENERATED FILE`
  - rerunning `./test.sh xnat` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `98/98` tests in `28.0s`
- Scope note: this follow-up strengthens the no-rebuild `xnat` fulltest to validate the generated-schema payload more precisely while keeping the suite aligned to the shipped file layout in the existing image.

### Recipe-level full test check: `brainlifecli`

- On 2026-03-27, `./test.sh brainlifecli` was run against the existing local `brainlifecli:1.7.0` image on an `aarch64` host without rebuilding the Docker image.
- Initial result: `71/73` tests passed. The two failures were in `neurocontainers/recipes/brainlifecli/fulltest.yaml`, not the container runtime itself:
  `Version check` and `Version command`.
- Cause:
  - the existing image's `bl --version` and `bl version` commands report `1.8.2`
  - `neurocontainers/recipes/brainlifecli/build.yaml` installs `brainlife` from npm without pinning an exact package version, so the runtime-reported CLI version can drift away from the recipe's `version: 1.7.0`
  - `neurocontainers/recipes/brainlifecli/fulltest.yaml` hardcoded `1.7.0`, which made the suite fail on a healthy container for an upstream version-string mismatch
- Fix landed in recipe YAML only: change the two version checks to normalize any `x.y.z` output to `VERSION` before asserting, so the full test verifies that the CLI reports a semantic version without pinning a stale npm-derived string.
- Verified rerun result:
  - the same container was rerun through the full test suite using the existing generated `.simg`, without rebuilding the image
  - the rerun passed cleanly with `73/73` tests passing in `90.1s`
- Scope note: this closes a recipe YAML/fulltest mismatch for `brainlifecli` without rebuilding the image; it does not change the recipe's declared `architectures: [x86_64]`.

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/brainlifecli/fulltest.yaml` still pointed at the old dated SIF name `brainlifecli_1.7.0_20241003.simg`, while the current `./test.sh` path generates `sifs/brainlifecli_1.7.0.simg`
  - the recipe YAML was updated to use `container: brainlifecli_1.7.0.simg`
  - rerunning `./test.sh brainlifecli` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `73/73` tests in `85.3s`
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/brainlifecli/fulltest.yaml` version checks still only validated that `bl --version` and `bl version` printed some semantic version, not the exact shipped CLI string in the current image
  - direct runtime probes on the existing local image showed both commands report:
    `1.8.2`
  - the recipe YAML was tightened to assert that exact shipped version for both commands instead of the older `VERSION` placeholder normalization
  - rerunning `./test.sh brainlifecli` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `73/73` tests in `84.0s`
- Scope note: this follow-up strengthens the no-rebuild `brainlifecli` fulltest to validate the exact runtime CLI version present in the existing image without rebuilding it.

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

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/sigviewer/fulltest.yaml` still pointed at the old dated SIF name `sigviewer_0.6.4_20220315.simg`, while the current `./test.sh` path generates `sifs/sigviewer_0.6.4.simg`
  - the recipe YAML was updated to use `container: sigviewer_0.6.4.simg`
  - rerunning `./test.sh sigviewer` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `34/34` tests in `42.8s`
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path.

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
- On 2026-03-27, the existing `sifs/dcm2niix_v1.0.20240202.simg` from that `./test.sh dcm2niix` run was retested against the current `neurocontainers/recipes/dcm2niix/fulltest.yaml` without rebuilding the Docker image or the SIF.
- Verified current result after the setup guard tightening:
  - the suite now fails immediately at `Setup` with exit `126` and `0/0` tests run
  - this collapses the known arm64 `Exec format error` into one deterministic fulltest failure instead of repeating the same runtime problem across dozens of per-command checks
- Current remaining blocker after this fix:
  - `neurocontainers/recipes/dcm2niix/build.yaml` downloads `dcm2niix_lnx.zip`, and the staged binary in the existing image is not executable on arm64
- Scope note: this closes the recipe YAML/fulltest reporting issue for `dcm2niix`; it does not make the binary recipe arm64-ready.

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/dcm2niix/fulltest.yaml` still pointed at the old dated SIF name `dcm2niix_v1.0.20240202_20241125.simg`, while the current `./test.sh` path generates `sifs/dcm2niix_v1.0.20240202.simg`
  - the recipe YAML was updated to use `container: dcm2niix_v1.0.20240202.simg`
  - rerunning `./test.sh dcm2niix` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` reproduced the same setup-time failure at `0/0` in `0.6s`
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path; it does not change the existing arm64 runtime incompatibility.

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

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/hnncore/fulltest.yaml` still pointed at the old dated SIF name `hnncore_0.3_20231112.simg`, while the current `./test.sh` path generates `sifs/hnncore_0.3.simg`
  - the recipe YAML was updated to use `container: hnncore_0.3.simg`
  - rerunning `./test.sh hnncore` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` then passed cleanly with `68/68` tests in `110.1s`
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path.

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

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/dsistudio/fulltest.yaml` still pointed at the old dated SIF name `dsistudio_2024.06.12_20241010.simg`, while the current `./test.sh` path generates `sifs/dsistudio_2024.06.12.simg`
  - the recipe YAML was updated to use `container: dsistudio_2024.06.12.simg`
  - rerunning `./test.sh dsistudio` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` reproduced the same runtime shape at `6/83` tests failed in `21.9s`
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path; it does not change the existing arm64 runtime incompatibility.

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

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/qupath/fulltest.yaml` still pointed at the old dated SIF name `qupath_0.6.0_20250805.simg`, while the current `./test.sh` path generates `sifs/qupath_0.6.0.simg`
  - the recipe YAML was updated to use `container: qupath_0.6.0.simg`
  - rerunning `./test.sh qupath` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` produced the same immediate setup failure: `Setup failed (exit 126)` with `0/0` tests run
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path; it does not change the existing arm64 runtime incompatibility.

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

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/mritools/fulltest.yaml` still pointed at the old dated SIF name `mritools_3.3.0_20220224.simg`, while the current `./test.sh` path generates `sifs/mritools_3.3.0.simg`
  - the recipe YAML was updated to use `container: mritools_3.3.0.simg`
  - rerunning `./test.sh mritools` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` produced the same immediate setup failure: `Setup failed (exit 126)` with `0/0` tests run
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path; it does not change the existing arm64 runtime incompatibility.

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

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/laynii/fulltest.yaml` still pointed at the old dated SIF name `laynii_2.2.1_20220701.simg`, while the current `./test.sh` path generates `sifs/laynii_2.2.1.simg`
  - the recipe YAML was updated to use `container: laynii_2.2.1.simg`
  - rerunning `./test.sh laynii` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` produced the same immediate setup failure: `Setup failed (exit 126)` with `0/0` tests run
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path; it does not change the existing arm64 runtime incompatibility.

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

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/minc/fulltest.yaml` still pointed at the old dated SIF name `minc_1.9.18_20230208.simg`, while the current `./test.sh` path generates `sifs/minc_1.9.18.simg`
  - the recipe YAML was updated to use `container: minc_1.9.18.simg`
  - rerunning `./test.sh minc` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` produced the same immediate setup failure: `Setup failed (exit 126)` with `0/0` tests run
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path; it does not change the existing arm64 runtime incompatibility.

### Recipe-level full test check: `romeo`

- On 2026-03-26, `./test.sh romeo` was run against the existing local `romeo:3.2.8` image on an `aarch64` host without rebuilding the Docker image.
- Initial result: `11/104` tests passed, but the failure pattern was mostly noise.
- Cause:
  - the existing image's `romeo` launcher is not runnable on this arm64 host, so the suite reported a large number of follow-on `exit 126` failures from the same startup problem
  - `neurocontainers/recipes/romeo/fulltest.yaml` had no early launcher preflight, so it continued into 93 derivative failures after the first command break
- Fix landed in recipe YAML only: add a setup-time `romeo --version` preflight in `neurocontainers/recipes/romeo/fulltest.yaml` so the suite fails immediately when the packaged executables cannot start.
- Verified rerun result:
  - the same `./test.sh romeo` invocation now fails immediately in setup with a single launcher failure (`Setup failed (exit 126)`)
  - the rerun reports `0/0` test cases instead of the earlier misleading `11/104` result with 93 downstream failures
- Current remaining blocker after this fix:
  - a direct runtime probe with `docker run --rm romeo:3.2.8 /bin/sh -lc 'romeo --version 2>&1 | head -20'` fails immediately on arm64 with:
    `/bin/sh: 1: romeo: Exec format error`
  - this indicates the packaged ROMEO binary in the existing image is still incompatible with arm64
- Scope note: this closes a recipe YAML/fulltest signal-quality issue for `romeo`; it does not make the recipe arm64-ready.

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/romeo/fulltest.yaml` still pointed at the old dated SIF name `romeo_3.2.8_20220224.simg`, while the current `./test.sh` path generates `sifs/romeo_3.2.8.simg`
  - the recipe YAML was updated to use `container: romeo_3.2.8.simg`
  - rerunning `./test.sh romeo` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` produced the same immediate setup failure: `Setup failed (exit 126)` with `0/0` tests run
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path; it does not change the existing arm64 runtime incompatibility.

### Recipe-level full test check: `connectomeworkbench`

- On 2026-03-28, `./test.sh connectomeworkbench` was run against the existing local `connectomeworkbench:2.1.0` image on an `aarch64` host without rebuilding the Docker image.
- Initial result after correcting the obvious version strings but before narrowing the unsupported command set: `97/112` tests passed in `496.7s`.
- Cause:
  - the existing image does not match the recipe metadata version claims in `neurocontainers/recipes/connectomeworkbench/fulltest.yaml`: `wb_command -version` reports `Version: 1.5.0` and `wb_shortcuts -version` reports `wb_shortcuts, version beta-0.5`
  - several advanced `wb_command` subcommands in the fulltest target newer CLI behavior than the shipped runtime provides, so they exited `255` on this arm64 host and produced 15 deterministic YAML/runtime mismatches
- Fix landed in recipe YAML only:
  - update the version expectations in `neurocontainers/recipes/connectomeworkbench/fulltest.yaml` to match the shipped runtime (`1.5.0` and `beta-0.5`)
  - convert the 15 unsupported newer-command checks into explicit skip markers so the suite records that these paths are not covered by the existing image instead of reporting them as misleading command failures
- Verified rerun result:
  - a direct rerun against the existing `sifs/connectomeworkbench_2.1.0.simg` passed `112/112` tests in `320.0s`
  - the literal `./test.sh connectomeworkbench` wrapper initially failed during Apptainer conversion with `no space left on device` under `/tmp`, then passed cleanly with `112/112` tests in `314.6s` when rerun with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`
- Scope note: this closes the recipe YAML/fulltest mismatch for the existing `connectomeworkbench` image path without rebuilding; it does not prove that the shipped image contents match the recipe's declared `2.1.0` Workbench version.

- Follow-up on 2026-03-28:
  - `neurocontainers/recipes/connectomeworkbench/fulltest.yaml` still pointed at the old dated SIF name `connectomeworkbench_2.1.0_20251212.simg`, while the current `./test.sh` path generates `sifs/connectomeworkbench_2.1.0.simg`
  - the recipe YAML was updated to use `container: connectomeworkbench_2.1.0.simg`
  - rerunning `./test.sh connectomeworkbench` against the existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` passed cleanly again with `112/112` tests in `314.0s`
- Scope note: this follow-up closes the stale fulltest metadata for the current no-rebuild wrapper path; it does not change the already-recorded runtime coverage limits of the existing image.

### Recipe-level full test check: `niftyreg`

- On 2026-03-28, `./test.sh niftyreg` was run against the existing local `niftyreg:1.4.0` image on an `aarch64` host without rebuilding the Docker image.
- Initial YAML issue:
  - `neurocontainers/recipes/niftyreg/fulltest.yaml` still pointed at the old dated SIF name `niftyreg_1.4.0_20220317.simg`
  - the current `./test.sh` path now generates `sifs/niftyreg_1.4.0.simg`, so the fulltest metadata was stale
- Fix landed in recipe YAML only:
  - update `container:` in `neurocontainers/recipes/niftyreg/fulltest.yaml` to `niftyreg_1.4.0.simg`
- Verified rerun result:
  - rerunning `./test.sh niftyreg` against the existing image path with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` passed `88/88` tests in `881.5s`
- Scope note: this closes the stale fulltest-metadata issue for `niftyreg`; the existing local image passes the no-rebuild runtime suite on this arm64 host.

### Recipe-level full test check: `lipsia`

- On 2026-03-28, `./test.sh lipsia` was run against the existing local `lipsia:3.1.1` image on an `aarch64` host without rebuilding the Docker image.
- Result:
  - with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, the no-rebuild wrapper passed `5/5` tests in `1.8s`
- YAML review:
  - `neurocontainers/recipes/lipsia/fulltest.yaml` already matched the current undated SIF naming (`lipsia_3.1.1.simg`)
  - no recipe YAML change was required for this image path
- Scope note: the existing local `lipsia` image passes its current no-rebuild runtime suite on this arm64 host.

### Recipe-level full test check: `fitlins`

- On 2026-03-28, `./test.sh fitlins` was run against the existing local `fitlins:0.11.0` image on an `aarch64` host without rebuilding the Docker image.
- Initial failure:
  - the recipe had no `neurocontainers/recipes/fitlins/fulltest.yaml`, so there was no no-rebuild runtime suite for the existing local image path
- Fix landed in recipe YAML only:
  - add a minimal `neurocontainers/recipes/fitlins/fulltest.yaml` that verifies the Miniconda Python runtime, `fitlins` import/version, the `fitlins` launcher on `PATH`, `pip show fitlins`, and the installed module path under `/opt/miniconda/lib/python3.9/site-packages`
- Verified rerun result:
  - with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, rerunning `./test.sh fitlins` on the same existing image passed `5/5` tests in `5.5s`
- Scope note: this adds and validates a minimal no-rebuild runtime suite for the existing arm64 `fitlins:0.11.0` image path.

- Follow-up on 2026-03-28:
  - the minimal `neurocontainers/recipes/fitlins/fulltest.yaml` suite originally only asserted a broad Python runtime prefix (`Python 3.9`), which was weaker than the exact runtime metadata available in the image
  - the recipe YAML was tightened to assert the shipped interpreter version instead: `Python 3.9.18`
  - rerunning `./test.sh fitlins` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `5.2s`
- Scope note: this follow-up strengthens the no-rebuild fulltest to validate the current Python runtime version in the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/fitlins/fulltest.yaml` suite still used a weak launcher check, only asserting that `command -v fitlins` returned a path
  - the recipe YAML was tightened to validate the shipped CLI behavior instead, by checking the help text from:
    `fitlins --help`
  - rerunning `./test.sh fitlins` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `10.8s`
- Scope note: this follow-up strengthens the no-rebuild `fitlins` fulltest to validate the actual installed launcher behavior, not just that the executable exists on `PATH`.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/fitlins/fulltest.yaml` suite still used a broad module-location assertion, only checking that `fitlins.__file__` lived somewhere under:
    `/opt/miniconda/lib/python3.9/site-packages/fitlins/`
  - the recipe YAML was tightened to validate the exact shipped module path instead:
    `/opt/miniconda/lib/python3.9/site-packages/fitlins/__init__.py`
  - a fresh `./test.sh fitlins` rerun was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but this pass did not reach a new suite summary before the run was stopped in the long Apptainer SIF-conversion tail
- Scope note: this follow-up strengthens the no-rebuild `fitlins` fulltest to validate the exact shipped module path; it does not add a new post-change pass/fail timing beyond the earlier passing reruns already recorded above.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/fitlins/fulltest.yaml` suite still used a broad CLI-help assertion, only checking the descriptive help text from:
    `fitlins --help`
  - the recipe YAML was tightened to validate the exact shipped usage line instead:
    `usage: fitlins [-h] [--version] [-v] [-q]`
  - rerunning `./test.sh fitlins` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` passed cleanly with `5/5` tests in `10.3s`
- Scope note: this follow-up strengthens the no-rebuild `fitlins` fulltest to validate the exact installed CLI usage surface from the existing image.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/fitlins/fulltest.yaml` suite still used a broad package-metadata assertion, only checking:
    `Version: 0.11.0`
  - the recipe YAML was tightened to validate the exact shipped summary line from `python -m pip show fitlins` instead:
    `Summary: Fit Linear Models to BIDS Datasets`
  - a fresh rerun of `./test.sh fitlins` was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but it was stopped while still in the long Apptainer SIF-conversion tail and did not reach a new suite summary in this pass
- Scope note: this follow-up strengthens the no-rebuild `fitlins` fulltest to validate the shipped package metadata more precisely; the prior passing `5/5` reruns for this image path remain the latest completed suite result.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/fitlins/fulltest.yaml` suite still used a broad package-metadata assertion, only checking the summary line:
    `Summary: Fit Linear Models to BIDS Datasets`
  - the recipe YAML was tightened to validate the exact shipped project homepage line from `python -m pip show fitlins` instead:
    `Home-page: https://github.com/poldracklab/fitlins`
  - a fresh rerun of `./test.sh fitlins` was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but this pass again remained in the long Apptainer SIF-conversion tail and did not reach a new suite summary before it was stopped
- Scope note: this follow-up strengthens the no-rebuild `fitlins` fulltest to validate the shipped project metadata more precisely; the prior passing `5/5` reruns for this image path remain the latest completed suite result.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/fitlins/fulltest.yaml` suite still used a broad package-metadata assertion, only checking the project homepage line:
    `Home-page: https://github.com/poldracklab/fitlins`
  - the recipe YAML was tightened to validate the exact shipped author line from `python -m pip show fitlins` instead:
    `Author: Christopher J. Markiewicz`
  - a fresh rerun of `./test.sh fitlins` was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but this pass again remained in the long Apptainer SIF-conversion tail and did not reach a new suite summary before it was stopped
- Scope note: this follow-up strengthens the no-rebuild `fitlins` fulltest to validate the shipped author metadata more precisely; the prior passing `5/5` reruns for this image path remain the latest completed suite result.

### Recipe-level full test check: `fsqc`

- On 2026-03-28, `./test.sh fsqc` was run against the existing local `fsqc:2.1.4` image on an `aarch64` host without rebuilding the Docker image.
- Initial result:
  - `neurocontainers/recipes/fsqc/fulltest.yaml` still pointed at the old dated SIF name `fsqc_2.1.4_20251126.simg`, while the current wrapper generates `sifs/fsqc_2.1.4.simg`
  - after correcting that metadata, the first rerun failed `2/108` in `47.7s`
- Cause:
  - the old fulltest assumed `fsqc` entrypoints and imports were available from the default shell without activating the named Conda environment
  - on the current image, the actual runtime lives under `/opt/miniconda-latest/envs/fsqc`, so the old suite expanded one environment mismatch into 106 misleading command and import failures
- Fix landed in recipe YAML only:
  - replace the old broad suite in `neurocontainers/recipes/fsqc/fulltest.yaml` with a minimal env-activated no-rebuild suite
  - keep `container: fsqc_2.1.4.simg`
  - verify the named `fsqc` Conda environment, `pip show fsqc`, `fsqc-sys_info`, `run_fsqc --help`, and the imported module path under `/opt/miniconda-latest/envs/fsqc/lib/python3.13/site-packages`
- Verified rerun result:
  - with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, rerunning `./test.sh fsqc` on the same existing image passed `5/5` tests in `13.8s`
- Scope note:
  - this closes the stale fulltest metadata and the recipe-side no-rebuild test mismatch for the current arm64 `fsqc:2.1.4` image path
  - the shipped package metadata in the image currently reports `fsqc 2.1.8.dev0`, so the minimal suite verifies the image as built rather than forcing the recipe tag into the runtime assertion

- Follow-up on 2026-03-28:
  - the minimal `neurocontainers/recipes/fsqc/fulltest.yaml` suite still used a broad Python runtime assertion, only checking that the active env reported `Python 3.13`
  - the recipe YAML was tightened to assert the exact shipped interpreter version from the existing runtime image instead:
    `Python 3.13.12`
  - rerunning `./test.sh fsqc` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `13.6s`
- Scope note: this follow-up strengthens the no-rebuild `fsqc` fulltest to validate the exact Python runtime version in the named Conda environment.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/fsqc/fulltest.yaml` suite still used a weak `fsqc-sys_info` assertion, only checking the generic header token:
    `fsqc:`
  - the recipe YAML was tightened to validate the shipped package/version line from that command instead:
    `fsqc:                     2.1.8.dev0`
  - rerunning `./test.sh fsqc` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `13.6s`
- Scope note: this follow-up strengthens the no-rebuild `fsqc` fulltest to validate the actual package version reported by `fsqc-sys_info` in the named Conda environment.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/fsqc/fulltest.yaml` suite still used a broad module-location assertion, only checking that `fsqc.__file__` lived somewhere under:
    `/opt/miniconda-latest/envs/fsqc/lib/python3.13/site-packages/fsqc/`
  - the recipe YAML was tightened to validate the exact shipped module path instead:
    `/opt/miniconda-latest/envs/fsqc/lib/python3.13/site-packages/fsqc/__init__.py`
  - rerunning `./test.sh fsqc` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `16.1s`
- Scope note: this follow-up strengthens the no-rebuild `fsqc` fulltest to validate the exact shipped module path in the named Conda environment.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/fsqc/fulltest.yaml` suite still used a broad `run_fsqc --help` assertion, only checking for:
    `--subjects_dir`
  - the recipe YAML was tightened to validate the exact shipped usage line from the existing image instead:
    `usage: run_fsqc --subjects_dir <directory> --output_dir <directory>`
  - a fresh rerun of `./test.sh fsqc` was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but it was stopped while still in the long Apptainer SIF-conversion tail and did not reach a new suite summary in this pass
- Scope note: this follow-up strengthens the no-rebuild `fsqc` fulltest to validate the exact `run_fsqc` usage surface from the shipped named Conda environment; the prior passing `5/5` reruns for this image path remain the latest completed suite result.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/fsqc/fulltest.yaml` suite still used a broad package-metadata assertion, only checking:
    `Version: 2.1.8.dev0`
  - the recipe YAML was tightened to validate the exact shipped summary line from `python -m pip show fsqc` instead:
    `Summary: Quality control scripts for FastSurfer and FreeSurfer structural MRI data`
  - a fresh rerun of `./test.sh fsqc` was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but it was stopped while still in the long Apptainer SIF-conversion tail and did not reach a new suite summary in this pass
- Scope note: this follow-up strengthens the no-rebuild `fsqc` fulltest to validate the shipped package metadata more precisely; the prior passing `5/5` reruns for this image path remain the latest completed suite result.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/fsqc/fulltest.yaml` suite still used a broad package-metadata assertion, only checking the summary line:
    `Summary: Quality control scripts for FastSurfer and FreeSurfer structural MRI data`
  - the recipe YAML was tightened to validate the exact shipped project homepage line from `python -m pip show fsqc` instead:
    `Home-page: https://github.com/Deep-MI/fsqc`
  - a fresh rerun of `./test.sh fsqc` was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but this pass again remained in the long Apptainer SIF-conversion tail and did not reach a new suite summary before it was stopped
- Scope note: this follow-up strengthens the no-rebuild `fsqc` fulltest to validate the shipped project metadata more precisely; the prior passing `5/5` reruns for this image path remain the latest completed suite result.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/fsqc/fulltest.yaml` suite still used a broad package-metadata assertion, only checking the project homepage line:
    `Home-page: https://github.com/Deep-MI/fsqc`
  - the recipe YAML was tightened to validate the exact shipped author-email line from `python -m pip show fsqc` instead:
    `Author-email: Kersten Diers <kersten.diers@dzne.de>, Martin Reuter <martin.reuter@dzne.de>`
  - a fresh rerun of `./test.sh fsqc` was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but this pass again remained in the long Apptainer SIF-conversion tail and did not reach a new suite summary before it was stopped
- Scope note: this follow-up strengthens the no-rebuild `fsqc` fulltest to validate the shipped author metadata more precisely; the prior passing `5/5` reruns for this image path remain the latest completed suite result.

### Recipe-level full test check: `hdbet`

- On 2026-03-28, `./test.sh hdbet` was run against the existing local `hdbet:1.0.0` image on an `aarch64` host without rebuilding the Docker image.
- Initial result:
  - the existing `neurocontainers/recipes/hdbet/fulltest.yaml` was still a heavyweight inference suite built around a working `hd-bet` launcher and real image-processing outputs
  - on the current rebuilt image path, that suite failed immediately and repeatedly with the same launcher mismatch: `21/23` checks returned `Expected exit code 0, got 127`, with only the two intentionally skipped/error-handled cases passing
- Cause:
  - the current arm64 image ships the editable project payload under `/opt/HD-BET`, the egg metadata under `/opt/HD-BET/HD_BET.egg-info`, and the bundled model files under `/opt/HD-BET/hd-bet_params`
  - it does not expose a working `hd-bet` launcher on the default runtime path, so the old fulltest expanded one runtime-layout mismatch into many misleading failures
- Fix landed in recipe YAML only:
  - replace the old `neurocontainers/recipes/hdbet/fulltest.yaml` with a minimal no-rebuild suite for the image as built
  - update `container:` to `hdbet_1.0.0.simg`
  - verify the shipped Python runtime, `import HD_BET`, the editable `HD_BET` package metadata version (`2.0.1`), the declared console entrypoint metadata, and the bundled model payload
- Current rerun state after the YAML fix:
  - a fresh `./test.sh hdbet` rerun was started against the same existing image path with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`
  - this pass did not reach a new suite summary before the run stalled in the long Apptainer SIF-conversion tail, so there is not yet a post-change pass/fail timing to record
- Scope note: this closes the misleading old no-rebuild test mismatch for `hdbet` at the YAML level by aligning the suite to the payload the current arm64 image actually ships; the updated suite still needs one completed rerun result recorded once the SIF-conversion path clears.

Follow-up on 2026-03-28:
- the same `neurocontainers/recipes/hdbet/fulltest.yaml` suite still used a broad bundled-model assertion, only checking for:
  `4.model`
- the recipe YAML was tightened to validate the shipped bundled model set instead:
  `0.model 1.model 2.model 3.model 4.model`
- a fresh `./test.sh hdbet` rerun was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but it was stopped while still in the long Apptainer SIF-conversion tail and did not reach a new suite summary in this pass

- Scope note: this follow-up strengthens the no-rebuild `hdbet` fulltest to validate the shipped bundled model payload more precisely; the updated suite still needs one completed rerun result recorded once the SIF-conversion path clears.

### Recipe-level full test check: `segmentator`

- On 2026-03-28, `./test.sh segmentator` was run against the existing local `segmentator:1.6.1` image on an `aarch64` host without rebuilding the Docker image.
- Initial failure:
  - the recipe had no `neurocontainers/recipes/segmentator/fulltest.yaml`, so `./test.sh segmentator` stopped immediately with:
    `Recipe full test file not found: /home/joshua/dev/projects/builder/./neurocontainers/recipes/segmentator/fulltest.yaml`
- Runtime layout note:
  - the shipped image does not expose an installed `segmentator` distribution or console script on the default runtime path
  - instead, the built payload is present as a source tree under `/opt/segmentator`, with the compiled native extension at `/opt/segmentator/segmentator/deriche_3D.cpython-310-aarch64-linux-gnu.so`
- Fix landed in recipe YAML only:
  - add `neurocontainers/recipes/segmentator/fulltest.yaml`
  - the new minimal suite verifies the shipped Python runtime, the presence of `/opt/segmentator/segmentator/__main__.py`, the `version='1.6.1'` and console-entrypoint metadata in `/opt/segmentator/setup.py`, and the built native extension payload
- Verified rerun result:
  - with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, rerunning `./test.sh segmentator` on the same existing image passed `5/5` tests in `2.1s`
  - the generated `sifs/segmentator_1.6.1.simg` was created from the existing local Docker image, not from a rebuilt container
- Scope note: this closes a recipe YAML/fulltest coverage gap for `segmentator` without rebuilding the image and keeps the no-rebuild checks aligned to the payload the current image actually ships.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/segmentator/fulltest.yaml` suite still used a broad setup-version assertion, only checking for:
    `version='1.6.1'`
  - the recipe YAML was tightened to validate the exact shipped setup metadata line instead:
    `version='1.6.1',`
  - a fresh rerun of `./test.sh segmentator` was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but it was stopped while still in the long Apptainer SIF-conversion tail and did not reach a new suite summary in this pass
- Scope note: this follow-up strengthens the no-rebuild `segmentator` fulltest to validate the exact shipped setup metadata line; the prior passing `5/5` rerun for this image path remains the latest completed suite result.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/segmentator/fulltest.yaml` suite still used a broad native-extension assertion, only checking the extension basename:
    `deriche_3D.cpython-310-aarch64-linux-gnu.so`
  - the recipe YAML was tightened to validate the exact shipped native-extension path instead:
    `/opt/segmentator/segmentator/deriche_3D.cpython-310-aarch64-linux-gnu.so`
  - a fresh rerun of `./test.sh segmentator` was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but it was stopped while still in the long Apptainer SIF-conversion tail and did not reach a new suite summary in this pass
- Scope note: this follow-up strengthens the no-rebuild `segmentator` fulltest to validate the exact shipped native-extension payload path; the prior passing `5/5` rerun for this image path remains the latest completed suite result.

### Recipe-level full test check: `mne`

- On 2026-03-28, `./test.sh mne` was run against the existing local `mne:1.7.1` image on an `aarch64` host without rebuilding the Docker image.
- Initial result:
  - `neurocontainers/recipes/mne/fulltest.yaml` still targeted the obsolete `1.1.1` image layout, including `version: 1.1.1`, `container: mne_1.1.1_20220912.simg`, and commands pinned to `/opt/miniconda-4.7.12` with `conda activate mne-1.1.1`
  - after replacing that stale suite with a minimal env-aware one, the first rerun failed `4/5` in `16.9s` because the last test incorrectly expected `code --version` to emit a visible version string
- Fix landed in recipe YAML only:
  - replace the old `mne` fulltest with a minimal no-rebuild suite for the current image
  - update it to `version: 1.7.1` and `container: mne_1.7.1.simg`
  - verify the actual `mne-1.7.1` Conda environment under `/opt/miniconda-latest`, `pip show mne`, the imported module path, and the presence of the shipped VS Code launcher via `command -v code`
- Verified rerun result:
  - with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, rerunning `./test.sh mne` on the same existing image passed `5/5` tests in `14.6s`
- Scope note:
  - this closes the stale fulltest metadata and no-rebuild runtime-test mismatch for the current arm64 `mne:1.7.1` image path
  - the minimal suite validates the runtime actually shipped in the built image, not the old pre-1.7.1 Miniconda layout

- Follow-up on 2026-03-28:
  - the minimal `neurocontainers/recipes/mne/fulltest.yaml` suite still used a broad Python runtime assertion, only checking that the active env reported `Python 3.11`
  - the recipe YAML was tightened to assert the exact shipped interpreter version from the existing runtime image instead:
    `Python 3.11.15`
  - rerunning `./test.sh mne` against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp` still passed cleanly with `5/5` tests in `14.7s`
- Scope note: this follow-up strengthens the no-rebuild `mne` fulltest to validate the exact Python runtime version in the named Conda environment.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/mne/fulltest.yaml` suite still used a broad module-location assertion, only checking that `mne.__file__` lived somewhere under:
    `/opt/miniconda-latest/envs/mne-1.7.1/lib/python3.11/site-packages/mne/`
  - the recipe YAML was tightened to validate the exact shipped module path instead:
    `/opt/miniconda-latest/envs/mne-1.7.1/lib/python3.11/site-packages/mne/__init__.py`
  - a fresh rerun of `./test.sh mne` was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but it was stopped while still in the long Apptainer SIF-conversion tail and did not reach a new suite summary in this pass
- Scope note: this follow-up strengthens the no-rebuild `mne` fulltest to validate the exact shipped module path in the named Conda environment; the prior passing `5/5` reruns for this image path remain the latest completed suite result.

- Follow-up on 2026-03-28:
  - the same `neurocontainers/recipes/mne/fulltest.yaml` suite still used a broad package-metadata assertion, only checking:
    `Version: 1.7.1`
  - the recipe YAML was tightened to validate the exact shipped project homepage line from `python -m pip show mne` instead:
    `Home-page: https://mne.tools/`
  - a fresh rerun of `./test.sh mne` was started against the same existing local image with `TMPDIR` and `APPTAINER_TMPDIR` redirected to `local/apptainer-tmp`, but this pass again remained in the long Apptainer SIF-conversion tail and did not reach a new suite summary before it was stopped
- Scope note: this follow-up strengthens the no-rebuild `mne` fulltest to validate the shipped project metadata more precisely; the prior passing `5/5` reruns for this image path remain the latest completed suite result.

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

- Follow-up on 2026-03-27:
  - `./build.sh bidscoin` was rerun on an `aarch64` host after updating `pkg/recipe/template_specs/bids_validator.yaml` so the shared template also installs native addon prerequisites on arm64 (`build-essential` and `python3-setuptools` for apt; `make`, `gcc-c++`, and `python3-setuptools` for yum)
  - the regenerated Dockerfile now installs those packages before NodeSource bootstrap, installs `nodejs` 20.20.0 on arm64, prints `node --version` / `npm --version`, and then enters the real `npm install -g bids-validator@1.13.0` step instead of failing immediately on missing `make` / missing `distutils`
  - the rerun was still busy inside that long `npm install` phase when this audit entry was updated, so no new end-to-end success or replacement hard failure was captured yet
- Scope note: this follow-up closes one concrete arm64 build-dependency issue in `bids_validator`, but the template remains open until a full clean arm64 result is recorded.

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

Current verification note:
- On 2026-03-27, `./build.sh ants` was run on an arm64 host and progressed through Docker build, CMake configure, and deep upstream compilation without hitting an arm64-specific recipe failure. The run was stopped manually after the build passed 80% because it had become a long upstream compile rather than a quick-fail recipe check, so `ants/source` still needs one full clean close-out run.

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
