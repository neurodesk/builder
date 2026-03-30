# ARM64 Port Progress

Derived from `ARM64_TEMPLATE_AUDIT.md` and reconciled against local artifacts under `local/local_logs` and `local/test-results`.

Status rules used here:

- `not-started`: no recorded build or full-test artifact was found
- `build-attempted`: build evidence exists, but no finalized image was recorded
- `built`: finalized image recorded, but no completed full-test result was recorded
- `tested`: a completed full-test result was recorded, but the recipe is not yet both built and passing in the available evidence
- `completed`: successful image build and a passing completed full-test result were both recorded

Status totals:

- `not-started`: 34
- `build-attempted`: 57
- `built`: 2
- `tested`: 11
- `completed`: 66
- `total`: 170

| Container | Status | Notes |
|---|---|---|
| `afib1` | `completed` | Successful local build log `local/local_logs/build_afib1.log`; ARM64 image `afib1:1.6.0` built and passing full-test artifact(s) `local/test-results/afib1-fulltest.json` recorded from the local arm64 SIF rerun |
| `afni` | `build-attempted` | Local build log `local/local_logs/build_afni.log` exists, but it does not record a finalized image |
| `amico` | `completed` | Arm64 image built and full test passed |
| `ants` | `tested` | Full test completed and failed immediately on arm64 launcher exec-format issue |
| `apptainer` | `completed` | Successful local build log `local/local_logs/build_apptainer.log` and passing full-test artifact(s) `local/test-results/apptainer-fulltest.json` |
| `arfiproc` | `completed` | Successful local build log `local/local_logs/build_arfiproc.log`; ARM64 image `arfiproc:1.0.0` built and passing full-test artifact(s) `local/test-results/arfiproc-fulltest.json` recorded from the local arm64 Docker rerun against the existing image, covering native `aarch64` execution, OpenRecon helpers, and the shipped `arfiproc` module import path |
| `ashs` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `aslprep` | `build-attempted` | Local build log `local/local_logs/build_aslprep.log` exists, but it does not record a finalized image |
| `b0map` | `completed` | Successful local build log `local/local_logs/build_b0map.log`; ARM64 image `b0map:1.0.0` built and passing full-test artifact(s) `local/test-results/b0map-fulltest.json` recorded from the local arm64 Docker rerun against the existing image, covering native `aarch64` execution, `bet2`, OpenRecon helpers, and the shipped `b0map` module import path |
| `bart` | `completed` | Successful local build log `local/local_logs/build_bart.log` and passing full-test artifact(s) `local/test-results/bart-fulltest.json` |
| `batchheudiconv` | `completed` | Successful local build log `local/local_logs/build_batchheudiconv.log` and passing full-test artifact(s) `local/test-results/batchheudiconv-fulltest.json` |
| `bidsappaa` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `bidsappbaracus` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `bidsappbrainsuite` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `bidsapphcppipelines` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `bidsappmrtrix3connectome` | `build-attempted` | Local build log `local/local_logs/build_bidsappmrtrix3connectome.log` exists, but it does not record a finalized image |
| `bidsapppymvpa` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `bidsappspm` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `bidscoin` | `completed` | Successful local build log `local/local_logs/build_bidscoin.log` and passing full-test artifact(s) `local/test-results/bidscoin-fulltest.json` |
| `bidsme` | `completed` | Successful local build log `local/local_logs/build_bidsme.log` and passing full-test artifact(s) `local/test-results/bidsme-fulltest.json` |
| `bidstools` | `completed` | Successful local build log `local/local_logs/build_bidstools.log`; ARM64 image `bidstools:1.0.4` rebuilt after replacing the broken ARM64 miniconda template with an explicit `aarch64` bootstrap and compiling a native `Bru2` from source, and passing full-test artifact(s) `local/test-results/bidstools-fulltest.json` were recorded from the local arm64 Docker rerun against the rebuilt image, covering native `aarch64` execution, packaged tool paths, `Bru2`, `dcm2niix`, `dcmdump`, and the shipped `heudiconv` environment path. The builder's bundled post-build tester still fails separately with an ARM64 helper `exec format error` |
| `blastct` | `completed` | Arm64 image built and full test passed |
| `blender` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `braid` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `brainager` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `brainlesion` | `completed` | Successful local build log `local/local_logs/build_brainlesion.log`; ARM64 image `brainlesion:1.0.0` built after replacing the broken ARM64 miniconda template path with an explicit `aarch64` Miniconda bootstrap and pip environment install, and passing full-test artifact(s) `local/test-results/brainlesion-fulltest.json` were recorded from the local arm64 Docker rerun against the rebuilt image, covering the packaged Python runtime, pinned package metadata, representative imports, `brats`, and the active conda environment path |
| `brainlifecli` | `completed` | Successful local build log `local/local_logs/build_brainlifecli.log` and passing full-test artifact(s) `local/test-results/brainlifecli-fulltest.json` |
| `brainstorm` | `build-attempted` | Local build log `local/local_logs/build_brainstorm.log` exists, but it does not record a finalized image |
| `builder` | `completed` | Successful local build log `local/local_logs/build_builder.log` and passing full-test artifact(s) `local/test-results/builder-fulltest.json` |
| `cat12` | `build-attempted` | Local build log `local/local_logs/build_cat12.log` exists, but it does not record a finalized image |
| `cbsb0stats` | `completed` | Successful local build log `local/local_logs/build_cbsb0stats.log` and passing full-test artifact(s) `local/test-results/cbsb0stats-fulltest.json` |
| `civet` | `build-attempted` | Local build log `local/local_logs/build_civet.log` exists, but it does not record a finalized image |
| `clearswi` | `completed` | Successful local build log `local/local_logs/build_clearswi.log` and passing full-test artifact(s) `local/test-results/clearswi-fulltest.json` |
| `clinica` | `build-attempted` | Local build log `local/local_logs/build_clinica.log` exists, but it does not record a finalized image |
| `clinicadl` | `build-attempted` | Local build log `local/local_logs/build_clinicadl.log` exists, but it does not record a finalized image |
| `code` | `completed` | Successful local build log `local/local_logs/build_code.log`; ARM64 image `code:240320` built and passing full-test artifact(s) `local/test-results/code-fulltest.json` recorded from the local arm64 Docker rerun |
| `condaenvs` | `build-attempted` | Build progressed to upstream source acquisition, but no finalized image was recorded |
| `conn` | `build-attempted` | Local build log `local/local_logs/build_conn.log` exists, but it does not record a finalized image |
| `connectomemapper3` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `connectomeworkbench` | `completed` | Successful local build log `local/local_logs/build_connectomeworkbench.log` and passing full-test artifact(s) `local/test-results/connectomeworkbench-fulltest.json` |
| `convert3d` | `tested` | Arm64 image built, but full test completed with x86_64-only runtime payload failures |
| `cosmomvpa` | `completed` | Successful local build log `local/local_logs/build_cosmomvpa.log` and passing full-test artifact(s) `local/test-results/cosmomvpa-fulltest.json` |
| `cpac` | `build-attempted` | Local build log `local/local_logs/build_cpac.log` exists, but it does not record a finalized image |
| `dafne` | `build-attempted` | Local build log `local/local_logs/build_dafne.log` exists, but it does not record a finalized image |
| `datalad` | `completed` | Successful local build log `local/local_logs/build_datalad.log` and passing full-test artifact(s) `local/test-results/datalad-fulltest.json` |
| `dcm2bids` | `completed` | Successful local build log `local/local_logs/build_dcm2bids.log` and passing full-test artifact(s) `local/test-results/dcm2bids-fulltest.json` |
| `dcm2niix` | `completed` | Successful local build log `local/local_logs/build_dcm2niix.log` and passing full-test artifact(s) `local/test-results/dcm2niix-fulltest.json` |
| `deepisles` | `build-attempted` | Local build log `local/local_logs/build_deepisles.log` exists, but it does not record a finalized image |
| `deeplabcut` | `build-attempted` | Local build log `local/local_logs/build_deeplabcut.log` exists, but it does not record a finalized image |
| `deepretinotopy` | `build-attempted` | Local build log `local/local_logs/build_deepretinotopy.log` exists, but it does not record a finalized image |
| `deepsif` | `build-attempted` | Local build log `local/local_logs/build_deepsif.log` exists, but it does not record a finalized image |
| `delphi` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `dicompare` | `completed` | Successful local build log `local/local_logs/build_dicompare.log` and passing full-test artifact(s) `local/test-results/dicompare-fulltest.json` |
| `dicomtools` | `completed` | Successful local build log `local/local_logs/build_dicomtools.log` and passing full-test artifact(s) `local/test-results/dicomtools-fulltest.json` |
| `diffusiontoolkit` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `dsistudio` | `tested` | Existing image full test completed and failed on arm64 runtime incompatibility |
| `eeglab` | `build-attempted` | Local build log `local/local_logs/build_eeglab.log` exists, but it does not record a finalized image |
| `eharmonize` | `completed` | Arm64 image built and full test passed |
| `elastix` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `esilpd` | `build-attempted` | Local build log `local/local_logs/build_esilpd.log` exists, but it does not record a finalized image |
| `exploreasl` | `build-attempted` | Local build log `local/local_logs/build_exploreasl.log` exists, but it does not record a finalized image |
| `ezbids` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `fastsurfer` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `fieldtrip` | `build-attempted` | Local build log `local/local_logs/build_fieldtrip.log` exists, but it does not record a finalized image |
| `fitlins` | `completed` | Arm64 image built and full test passed |
| `fmriprep` | `build-attempted` | Local build log `local/local_logs/build_fmriprep.log` exists, but it does not record a finalized image |
| `freesurfer` | `build-attempted` | Local build log `local/local_logs/build_freesurfer.log` exists, but it does not record a finalized image |
| `fsl` | `tested` | Existing image full test completed and failed on amd64/arm64 mismatch |
| `fsqc` | `completed` | Arm64 image built and full test passed |
| `gigaconnectome` | `build-attempted` | Local build log `local/local_logs/build_gigaconnectome.log` exists, but it does not record a finalized image |
| `gimp` | `completed` | Successful local build log `local/local_logs/build_gimp.log` and passing full-test artifact(s) `local/test-results/gimp-fulltest.json` |
| `gingerale` | `completed` | Arm64 image built and full test passed |
| `glmsingle` | `completed` | Successful local build log `local/local_logs/build_glmsingle.log`; ARM64 image `glmsingle:1.2` built and passing full-test artifact(s) `local/test-results/glmsingle-fulltest.json` recorded after aligning the suite with the packaged metadata and generated Apptainer labels |
| `gouhfi` | `build-attempted` | Local build log `local/local_logs/build_gouhfi.log` exists, but it does not record a finalized image |
| `halfpipe` | `build-attempted` | Local build log `local/local_logs/build_halfpipe.log` exists, but it does not record a finalized image |
| `hcpasl` | `build-attempted` | Local build log `local/local_logs/build_hcpasl.log` exists, but it does not record a finalized image |
| `hdbet` | `completed` | Existing ARM64 Docker image `hdbet:1.0.0` (`docker image inspect` reports `arm64/linux`, created `2026-03-28T08:41:06.542938062+10:00`) now has a passing completed full-test artifact `local/test-results/hdbet-fulltest.json`; the prior failure was a stale full-test mismatch that exercised the system Python instead of the shipped `hdbet` conda environment |
| `heudiconv` | `completed` | Successful local build log `local/local_logs/build_heudiconv.log` and passing full-test artifact(s) `local/test-results/heudiconv-fulltest.json` |
| `hmri` | `build-attempted` | Local build log `local/local_logs/build_hmri.log` exists, but it does not record a finalized image |
| `hnncore` | `completed` | Successful local build log `local/local_logs/build_hnncore.log` and passing full-test artifact(s) `local/test-results/hnncore-fulltest.json` |
| `ilastik` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `irkernel` | `completed` | Successful local build log `local/local_logs/build_irkernel.log`; ARM64 image `irkernel:4.4.3` built and passing full-test artifact(s) `local/test-results/irkernel-fulltest.json` recorded from the local arm64 Docker rerun |
| `itksnap` | `tested` | Existing image full test completed and failed immediately on arm64 launcher exec-format issue |
| `jamovi` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `julia` | `completed` | Successful local build log `local/local_logs/build_julia.log`; ARM64 image `julia:1.9.4` built and passing full-test artifact(s) `local/test-results/julia-fulltest.json` recorded from the local arm64 Docker rerun |
| `laynii` | `tested` | Existing image full test completed and failed immediately on arm64 launcher exec-format issue |
| `lesionquantificationtoolkit` | `completed` | Successful local build log `local/local_logs/build_lesionquantificationtoolkit.log`; ARM64 image `lesionquantificationtoolkit:0.1.0` rebuilt after installing the missing R dependency path (`gsl` 2.1-8 and `aws`) so `LQT` installs cleanly, and passing full-test artifact(s) `local/test-results/lesionquantificationtoolkit-fulltest.json` were recorded from the local arm64 Docker rerun against the rebuilt image |
| `lesymap` | `build-attempted` | Local build log `local/local_logs/build_lesymap.log` exists, but it does not record a finalized image |
| `linda` | `build-attempted` | Local build log `local/local_logs/build_linda.log` exists, but it does not record a finalized image |
| `lipsia` | `completed` | Successful local build log `local/local_logs/build_lipsia.log` and passing full-test artifact(s) `local/test-results/lipsia-fulltest.json` |
| `lqt` | `completed` | Successful local build log `local/local_logs/build_lqt.log` and passing full-test artifact(s) `local/test-results/lqt-fulltest.json` |
| `lstai` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `megnet` | `build-attempted` | Build progressed into package solving, but no finalized image was recorded |
| `metabody` | `build-attempted` | Local build log `local/local_logs/build_metabody.log` exists, but it does not record a finalized image |
| `mgltools` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `micapipe` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `minc` | `tested` | Existing image full test completed and failed immediately on arm64 launcher exec-format issue |
| `mipav` | `build-attempted` | Build attempt reached bundled x86_64 JRE exec-format failure |
| `mitkdiffusion` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `mne` | `completed` | Arm64 image built and full test passed |
| `mricrogl` | `tested` | Image built, but full test completed with bundled dcm2niix arm64 incompatibility |
| `mricron` | `build-attempted` | Local build log `local/local_logs/build_mricron.log` exists, but it does not record a finalized image |
| `mriqc` | `build-attempted` | Local build log `local/local_logs/build_mriqc.log` exists, but it does not record a finalized image |
| `mritools` | `tested` | Existing image full test completed and failed immediately on arm64 launcher incompatibility |
| `mrsimetabolicconnectome` | `completed` | Successful local build log `local/local_logs/build_mrsimetabolicconnectome.log`; ARM64 image `mrsimetabolicconnectome:1.0.0` (`docker image inspect` reports `arm64/linux`, created `2026-03-31T01:21:23.291549924+10:00`) now has a passing completed full-test artifact `local/test-results/mrsimetabolicconnectome-fulltest.json` recorded via `./test.sh mrsimetabolicconnectome`, covering native `aarch64` execution, Python 3.10.20, installed `mrsitoolbox` metadata, the shipped preprocessing repository path, `registration_mrsi_to_t1.py --help`, and the core script import path (`Registration`, `DataUtils`, `BiHarmonic`) |
| `mrtrix3` | `build-attempted` | Local build log `local/local_logs/build_mrtrix3.log` exists, but it does not record a finalized image |
| `mrtrix3tissue` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `musclemap` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `networkcorrespondancetoolkit` | `completed` | Successful local build log `local/local_logs/build_networkcorrespondancetoolkit.log`; ARM64 image `networkcorrespondancetoolkit:0.3.3` built after enabling `aarch64`, switching the conda bootstrap to the ARM64 installer, replacing the unsatisfiable upstream env solve with a minimal Python 3.11 env, and relaxing the unavailable `vtk==9.3.0` pin to an ARM64-available wheel path; passing full-test artifact(s) `local/test-results/networkcorrespondancetoolkit-fulltest.json` recorded from the local arm64 Docker rerun against the existing image, covering native `aarch64` execution, Python 3.11, installed package metadata, module discovery, and the ARM64-sensitive dependency set including `vtk` |
| `neurocommand` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `neurodock` | `build-attempted` | Local build log `local/local_logs/build_neurodock.log` exists, but it does not record a finalized image |
| `nftsim` | `completed` | Successful local build log `local/local_logs/build_nftsim.log`; ARM64 image `nftsim:1.0.2` (`docker image inspect` reports `arm64/linux`, created `2026-03-31T02:01:40.04722515+10:00`) now has a passing completed full-test artifact `local/test-results/nftsim-fulltest.json` recorded via `./test.sh nftsim`, covering native `aarch64` execution, the rebuilt `nftsim` CLI help path, current `/opt/nftsim/configs` layout, and two source-tree example simulations (`test_time-validation.conf` and `e-cortical.conf`) |
| `nibabies` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `niftyreg` | `completed` | Arm64 image built and full test passed |
| `nighres` | `completed` | Successful local build log `local/local_logs/build_nighres.log`; ARM64 image `nighres:1.5.2` (`docker image inspect` reports `arm64/linux`, created `2026-03-31T00:58:24.521886596+10:00`) now has a passing completed full-test artifact `local/test-results/nighres-fulltest.json` recorded via `./test.sh nighres`, covering native `aarch64` execution, Python 3.12.3, installed `nighres` metadata, `jcc`/`nighresjava` imports, visible top-level modules, and a synthetic `nighres.io` NIfTI roundtrip |
| `niimath` | `completed` | Successful local build log `local/local_logs/build_niimath.log` and passing full-test artifact(s) `local/test-results/niimath-fulltest.json` |
| `niistat` | `completed` | Successful local build log `local/local_logs/build_niistat.log`; ARM64 image `niistat:1.0.20191216` built and passing full-test artifact(s) `local/test-results/niistat-fulltest.json` recorded from the local arm64 Apptainer rerun against `neurocontainers/sifs/niistat_1.0.20191216_20220111.simg` |
| `nipype` | `build-attempted` | Local build log `local/local_logs/build_nipype.log` exists, but it does not record a finalized image |
| `noddi` | `build-attempted` | Local build log `local/local_logs/build_noddi.log` exists, but it does not record a finalized image |
| `openads` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `openreconexample` | `completed` | Successful local build log `local/local_logs/build_openreconexample.log` and passing full-test artifact(s) `local/test-results/openreconexample-fulltest.json` |
| `oshyx` | `build-attempted` | Local build log `local/local_logs/build_oshyx.log` exists, but it does not record a finalized image |
| `osprey` | `build-attempted` | Local build log `local/local_logs/build_osprey.log` exists, but it does not record a finalized image |
| `ospreybids` | `build-attempted` | Local build log `local/local_logs/build_ospreybids.log` exists, but it does not record a finalized image |
| `palm` | `completed` | Successful local build log `local/local_logs/build_palm.log` and passing full-test artifact(s) `local/test-results/palm-fulltest.json` |
| `palmettobug` | `completed` | Successful local build log `local/local_logs/build_palmettobug.log`; ARM64 image `palmettobug:0.0.3` built after replacing the x86-only Miniconda path with a multi-arch Python 3.10 base, relaxing the unavailable ARM64 `PySide6<6.5` pin to `PySide6>=6.5.3,<6.6`, adding the native toolchain plus OpenBLAS for source-built `numcodecs`/`fdasrsf`, and restoring the Tk runtime required for the GUI import path; passing full-test artifact(s) `local/test-results/palmettobug-fulltest.json` recorded from the local arm64 rerun against the rebuilt image, covering native `aarch64` execution, Python 3.10, installed `palmettobug` package metadata, and representative `scanpy`/`squidpy` workflows |
| `pcntoolkit` | `completed` | Successful local build log `local/local_logs/build_pcntoolkit.log`; ARM64 image `pcntoolkit:0.35` built and passing full-test artifact `local/test-results/pcntoolkit-fulltest.json` recorded via the shared docker workflow on arm64 |
| `petprep` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `physio` | `build-attempted` | Local build log `local/local_logs/build_physio.log` exists, but it does not record a finalized image |
| `prostatefiducialseg` | `build-attempted` | Local build log `local/local_logs/build_prostatefiducialseg.log` exists, but it does not record a finalized image |
| `pydeface` | `build-attempted` | Local build log `local/local_logs/build_pydeface.log` exists, but it does not record a finalized image |
| `qsiprep` | `build-attempted` | Local build log `local/local_logs/build_qsiprep.log` exists, but it does not record a finalized image |
| `qsirecon` | `build-attempted` | Local build log `local/local_logs/build_qsirecon.log` exists, but it does not record a finalized image |
| `qsmbly` | `completed` | Successful local build log `local/local_logs/build_qsmbly.log` and passing full-test artifact(s) `local/test-results/qsmbly-fulltest.json` |
| `qsmxt` | `build-attempted` | Local build log `local/local_logs/build_qsmxt.log` exists, but it does not record a finalized image |
| `quickshear` | `completed` | Successful local build log `local/local_logs/build_quickshear.log`; ARM64 image `quickshear:1.1.0` (`docker image inspect` reports `arm64/linux`, created `2026-03-31T00:28:43.157660172+10:00`) now has a passing completed full-test artifact `local/test-results/quickshear-fulltest.json` recorded via `./test.sh quickshear`, covering native `aarch64` execution, Python 3.12.3, the shipped `quickshear` CLI/package metadata, and a synthetic end-to-end defacing run |
| `qupath` | `tested` | Existing image full test completed and failed immediately on arm64 launcher issue |
| `rabies` | `build-attempted` | Local build log `local/local_logs/build_rabies.log` exists, but it does not record a finalized image |
| `radtract` | `completed` | Successful local build log `local/local_logs/build_radtract.log` and passing full-test artifact(s) `local/test-results/radtract-fulltest.json` |
| `romeo` | `tested` | Existing image full test completed and failed immediately on arm64 launcher exec-format issue |
| `root` | `build-attempted` | Local build log `local/local_logs/build_root.log` exists, but it does not record a finalized image |
| `rshrf` | `completed` | Successful local build log `local/local_logs/build_rshrf.log`; ARM64 image `rshrf:1.5.8` built and passing full-test artifact(s) `local/test-results/rshrf-fulltest.json` recorded from the local arm64 Docker rerun |
| `rstudio` | `build-attempted` | Local build log `local/local_logs/build_rstudio.log` exists, but it does not record a finalized image |
| `samsrfx` | `build-attempted` | Local build log `local/local_logs/build_samsrfx.log` exists, but it does not record a finalized image |
| `segmentator` | `completed` | Arm64 image built and full test passed |
| `sigviewer` | `completed` | Successful local build log `local/local_logs/build_sigviewer.log` and passing full-test artifact(s) `local/test-results/sigviewer-fulltest.json` |
| `slicer` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `slicersalt` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `soopct` | `completed` | Successful local build log `local/local_logs/build_soopct.log`; ARM64 image `soopct:0.0.0` built after enabling `aarch64`, adding the native build toolchain needed for the `antspyx` fallback path, pinning the available ARM64 wheel route (`python=3.12`, `antspyx==0.4.2`), and compiling a native `dcm2niix` for the core DICOM conversion path; passing full-test artifact(s) `local/test-results/soopct-fulltest.json` recorded from the local arm64 rerun against the rebuilt image, covering native `aarch64` execution, Python 3.12, bundled sample DICOM conversion, best-volume selection, BIDS structuring, and mean-image generation |
| `spant` | `completed` | Successful local build log `local/local_logs/build_spant.log`; ARM64 image `spant:3.7.0` built and passing full-test artifact(s) `local/test-results/spant-fulltest.json` recorded from the local arm64 Docker rerun |
| `spinalcordtoolbox` | `build-attempted` | Local build log `local/local_logs/build_spinalcordtoolbox.log` exists, but it does not record a finalized image |
| `spm12` | `build-attempted` | Local build log `local/local_logs/build_spm12.log` exists, but it does not record a finalized image |
| `spm25` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `spmpython` | `completed` | Successful local build log `local/local_logs/build_spmpython.log` and passing full-test artifact(s) `local/test-results/spmpython-fulltest.json` |
| `surfice` | `build-attempted` | Local build log `local/local_logs/build_surfice.log` exists, but it does not record a finalized image |
| `syncro` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `template` | `completed` | Successful local build log `local/local_logs/build_template.log` and passing full-test artifact(s) `local/test-results/template-fulltest.json` |
| `terastitcher` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `tinyrange` | `completed` | Successful local build log `local/local_logs/build_tinyrange.log`; ARM64 image `tinyrange:0.4.0` built and passing full-test artifact(s) `local/test-results/tinyrange-fulltest.json` recorded from the local arm64 Docker rerun |
| `topaz` | `completed` | Successful local build log `local/local_logs/build_topaz.log` and passing full-test artifact(s) `local/test-results/topaz-fulltest.json` |
| `totalsegmentator` | `completed` | Successful local build log `local/local_logs/build_totalsegmentator.log`; ARM64 image `totalsegmentator:2.5.0` built and passing full-test artifact(s) `local/test-results/totalsegmentator-fulltest.json` recorded from the local arm64 Docker rerun |
| `trackvis` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `tractseg` | `build-attempted` | Local build log `local/local_logs/build_tractseg.log` exists, but it does not record a finalized image |
| `vesselboost` | `build-attempted` | Local build log `local/local_logs/build_vesselboost.log` exists, but it does not record a finalized image |
| `vesselvio` | `build-attempted` | Build progressed into the later PyQt build path, but no finalized image was recorded |
| `vina` | `completed` | Successful local build log `local/local_logs/build_vina.log`; ARM64 image `vina:1.2.3` (`docker image inspect` reports `arm64/linux`, created `2026-03-31T02:12:49.095138233+10:00`) now has a passing completed full-test artifact `local/test-results/vina-fulltest.json` recorded via `./test.sh vina`, covering native `aarch64` execution, Ubuntu package metadata for `autodock`/`autodock-vina`, the packaged `vina`/`vina_split`/`autodock4` binaries, and a basic `vina_split` PDBQT split run |
| `voreen` | `built` | Local ARM64 Docker image `voreen:5.3.0` is present (`docker image inspect` reports `arm64/linux`, created `2026-03-29T22:43:56.182579304+10:00`), but no completed full-test artifact is recorded under `local/test-results` |
| `workshopdemo` | `completed` | Successful local build log `local/local_logs/build_workshopdemo.log` and passing full-test artifact(s) `local/test-results/workshopdemo-fulltest.json` |
| `xcpd` | `build-attempted` | Local build log `local/local_logs/build_xcpd.log` exists, but it does not record a finalized image |
| `xnat` | `completed` | Successful local build log `local/local_logs/build_xnat.log` and passing full-test artifact(s) `local/test-results/xnat-fulltest.json` |
