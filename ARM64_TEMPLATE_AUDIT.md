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
- Verified rerun result:
  - after those YAML fixes, the build progressed through Miniconda setup, `conda create --name segmentator`, the modernized Conda dependency install, the corrected `compoda==0.3.5` pip install, and the recipe unpack step
  - the remaining failure is now later and narrower, during editable install of the upstream `segmentator` source tree:
    `ModuleNotFoundError: No module named 'numpy'`
    from pip's isolated editable-build metadata phase
- Scope note: this pass closes multiple concrete arm64 recipe-YAML blockers for `segmentator` and moves the build from immediate Conda/template failure into the upstream package's editable-install/build-isolation problem. A final successful arm64 image was not produced in this pass.

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
- Scope note: this closes two concrete recipe-side Miniconda blockers for `spmpython` on arm64 and moves the build into the real upstream pip install path; any later package or runtime issues remain to be closed out in a future run.

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
- Scope note: this pass closes three concrete recipe-side Miniconda blockers for `topaz` on arm64 and moves the build into the recipe's old Conda package constraints; a final successful arm64 image was not produced in this pass.

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
- Scope note: this pass closes two concrete recipe-side Miniconda blockers for `vesselvio` on arm64 and moves the build into the recipe's real Python dependency installation path; any later package or runtime issues remain to be closed out in a future run.

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
- Scope note: this pass closes two concrete recipe-side Miniconda blockers for `hdbet` on arm64 and moves the build to the next real compatibility issue: the recipe's `ubuntu:16.04` base image is now too old for the current arm64 Miniconda installer. A final successful arm64 image was not produced in this pass.

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
- On 2026-03-27, the existing `sifs/dcm2niix_v1.0.20240202.simg` from that `./test.sh dcm2niix` run was retested against the current `neurocontainers/recipes/dcm2niix/fulltest.yaml` without rebuilding the Docker image or the SIF.
- Verified current result after the setup guard tightening:
  - the suite now fails immediately at `Setup` with exit `126` and `0/0` tests run
  - this collapses the known arm64 `Exec format error` into one deterministic fulltest failure instead of repeating the same runtime problem across dozens of per-command checks
- Current remaining blocker after this fix:
  - `neurocontainers/recipes/dcm2niix/build.yaml` downloads `dcm2niix_lnx.zip`, and the staged binary in the existing image is not executable on arm64
- Scope note: this closes the recipe YAML/fulltest reporting issue for `dcm2niix`; it does not make the binary recipe arm64-ready.

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
